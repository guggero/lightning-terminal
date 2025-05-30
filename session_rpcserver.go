package terminal

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/ecdsa"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/lightninglabs/lightning-node-connect/mailbox"
	"github.com/lightninglabs/lightning-terminal/accounts"
	"github.com/lightninglabs/lightning-terminal/autopilotserver"
	"github.com/lightninglabs/lightning-terminal/firewall"
	"github.com/lightninglabs/lightning-terminal/firewalldb"
	"github.com/lightninglabs/lightning-terminal/litrpc"
	litmac "github.com/lightninglabs/lightning-terminal/macaroons"
	"github.com/lightninglabs/lightning-terminal/perms"
	"github.com/lightninglabs/lightning-terminal/rules"
	"github.com/lightninglabs/lightning-terminal/session"
	"github.com/lightningnetwork/lnd/fn"
	"github.com/lightningnetwork/lnd/macaroons"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"gopkg.in/macaroon-bakery.v2/bakery"
	"gopkg.in/macaroon-bakery.v2/bakery/checkers"
	"gopkg.in/macaroon.v2"
)

// readOnlyAction defines the keyword that a permission action should be set to
// when the entity is set to "uri" in order to activate the special case that
// will result in all read-only permissions known to lit to be added to a
// session's macaroon. The purpose of the three '*'s is to make this keyword
// an invalid URI and an invalid regex so that it does not ever clash with the
// other special cases.
const readOnlyAction = "***readonly***"

// sessionRpcServer is the gRPC server for the Session RPC interface.
type sessionRpcServer struct {
	litrpc.UnimplementedSessionsServer
	litrpc.UnimplementedFirewallServer
	litrpc.UnimplementedAutopilotServer

	cfg           *sessionRpcServerConfig
	sessionServer *session.Server

	quit     chan struct{}
	wg       sync.WaitGroup
	stopOnce sync.Once
}

// sessionRpcServerConfig holds the values used to configure the
// sessionRpcServer.
type sessionRpcServerConfig struct {
	db                      session.Store
	basicAuth               string
	grpcOptions             []grpc.ServerOption
	registerGrpcServers     func(server *grpc.Server)
	superMacBaker           litmac.Baker
	firstConnectionDeadline time.Duration
	permMgr                 *perms.Manager
	actionsDB               firewalldb.ActionDB
	autopilot               autopilotserver.Autopilot
	ruleMgrs                rules.ManagerSet
	privMap                 firewalldb.PrivacyMapper
}

// newSessionRPCServer creates a new sessionRpcServer using the passed config.
func newSessionRPCServer(cfg *sessionRpcServerConfig) (*sessionRpcServer,
	error) {

	// Create the gRPC server that handles adding/removing sessions and the
	// actual mailbox server that spins up the Terminal Connect server
	// interface.
	server := session.NewServer(
		func(id session.ID, opts ...grpc.ServerOption) *grpc.Server {
			// Add the session ID injector interceptors first so
			// that the session ID is available in the context of
			// all interceptors that come after.
			allOpts := []grpc.ServerOption{
				addSessionIDToStreamCtx(id),
				addSessionIDToUnaryCtx(id),
			}

			allOpts = append(allOpts, cfg.grpcOptions...)
			allOpts = append(allOpts, opts...)

			// Construct the gRPC server with the options.
			grpcServer := grpc.NewServer(allOpts...)

			// Register various grpc servers with the LNC session
			// server.
			cfg.registerGrpcServers(grpcServer)

			return grpcServer
		},
	)

	return &sessionRpcServer{
		cfg:           cfg,
		sessionServer: server,
		quit:          make(chan struct{}),
	}, nil
}

// wrappedServerStream is a wrapper around the grpc.ServerStream that allows us
// to set a custom context. This is needed since the stream handler function
// doesn't take a context as an argument, but rather has a Context method on the
// handler itself. So we use this custom wrapper to override this method.
type wrappedServerStream struct {
	grpc.ServerStream
	ctx context.Context
}

// Context returns the context of the stream.
//
// NOTE: This implements the grpc.ServerStream Context method.
func (w *wrappedServerStream) Context() context.Context {
	return w.ctx
}

// addSessionIDToStreamCtx is a gRPC stream interceptor that adds the given
// session ID to the context of the stream. This allows us to access the
// session ID later on for any gRPC calls made through this stream.
func addSessionIDToStreamCtx(id session.ID) grpc.ServerOption {
	return grpc.StreamInterceptor(func(srv any, ss grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler) error {

		md, _ := metadata.FromIncomingContext(ss.Context())
		mdCopy := md.Copy()
		session.AddToGRPCMetadata(mdCopy, id)

		// Wrap the original stream with our custom context.
		wrapped := &wrappedServerStream{
			ServerStream: ss,
			ctx: metadata.NewIncomingContext(
				ss.Context(), mdCopy,
			),
		}

		return handler(srv, wrapped)
	})
}

// addSessionIDToUnaryCtx is a gRPC unary interceptor that adds the given
// session ID to the context of the unary call. This allows us to access the
// session ID later on for any gRPC calls made through this context.
func addSessionIDToUnaryCtx(id session.ID) grpc.ServerOption {
	return grpc.UnaryInterceptor(func(ctx context.Context, req any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler) (resp any, err error) {

		md, _ := metadata.FromIncomingContext(ctx)
		mdCopy := md.Copy()
		session.AddToGRPCMetadata(mdCopy, id)

		return handler(metadata.NewIncomingContext(ctx, mdCopy), req)
	})
}

// start all the components necessary for the sessionRpcServer to start serving
// requests. This includes resuming all non-revoked sessions.
func (s *sessionRpcServer) start(ctx context.Context) error {
	// Delete all sessions in the Reserved state.
	err := s.cfg.db.DeleteReservedSessions(ctx)
	if err != nil {
		return fmt.Errorf("error deleting reserved sessions: %v", err)
	}

	// Start up all previously created sessions.
	sessions, err := s.cfg.db.ListSessionsByState(ctx, session.StateCreated)
	if err != nil {
		return fmt.Errorf("error listing sessions: %v", err)
	}

	// For backwards compatibility, we will also resume sessions that are in
	// the InUse state even though we no longer put sessions into this
	// state.
	inUseSessions, err := s.cfg.db.ListSessionsByState(
		ctx, session.StateInUse,
	)
	if err != nil {
		return fmt.Errorf("error listing sessions: %v", err)
	}

	sessions = append(sessions, inUseSessions...)

	for _, sess := range sessions {
		key := sess.LocalPublicKey.SerializeCompressed()

		if sess.Type == session.TypeAutopilot {
			// We only start the autopilot sessions if the autopilot
			// client has been enabled.
			if s.cfg.autopilot == nil {
				continue
			}

			// Do a sanity check to ensure that we have the static
			// remote pub key stored for this session. This should
			// never not be the case.
			if sess.RemotePublicKey == nil {
				log.Errorf("no static remote key found for "+
					"autopilot session %x", key)

				continue
			}

			if sess.Expiry.Before(time.Now()) {
				continue
			}

			ctxc, cancel := context.WithTimeout(
				ctx, defaultConnectTimeout,
			)

			// Register the session with the autopilot client.
			perm, err := s.cfg.autopilot.ActivateSession(
				ctxc, sess.LocalPublicKey,
			)
			cancel()
			if err != nil {
				log.Errorf("error activating autopilot "+
					"session (%x) with the client: %v", key,
					err)

				if perm {
					err := s.cfg.db.ShiftState(
						ctx, sess.ID, session.StateRevoked,
					)
					if err != nil {
						log.Errorf("error revoking "+
							"session: %v", err)
					}

					continue
				}
			}
		}

		if err := s.resumeSession(ctx, sess); err != nil {
			log.Errorf("error resuming session (%x): %v", key, err)
		}
	}

	return nil
}

// stop cleans up any sessionRpcServer resources.
func (s *sessionRpcServer) stop() error {
	var returnErr error
	s.stopOnce.Do(func() {
		s.sessionServer.Stop()

		close(s.quit)
		s.wg.Wait()
	})

	return returnErr
}

// AddSession adds and starts a new Terminal Connect session.
func (s *sessionRpcServer) AddSession(ctx context.Context,
	req *litrpc.AddSessionRequest) (*litrpc.AddSessionResponse, error) {

	expiry := time.Unix(int64(req.ExpiryTimestampSeconds), 0)
	if time.Now().After(expiry) {
		return nil, fmt.Errorf("expiry must be in the future")
	}

	typ, err := unmarshalRPCType(req.SessionType)
	if err != nil {
		return nil, err
	}

	// Store the entity-action permission pairs in a map in order to
	// de-dup any repeat perms.
	permissions := make(map[string]map[string]struct{})

	// addPerm is a closure that can be used to add entity-action pairs to
	// the permissions map.
	addPerm := func(entity, action string) {
		_, ok := permissions[entity]
		if !ok {
			permissions[entity] = make(map[string]struct{})
		}

		permissions[entity][action] = struct{}{}
	}

	var (
		caveats   []macaroon.Caveat
		accountID fn.Option[accounts.AccountID]
	)
	switch typ {
	// For the default session types we use empty caveats and permissions,
	// the macaroons are baked correctly when creating the session.
	case session.TypeMacaroonAdmin, session.TypeMacaroonReadonly:

	// For account based sessions we just add the account ID caveat, the
	// permissions are added dynamically when creating the session.
	case session.TypeMacaroonAccount:
		id, err := accounts.ParseAccountID(req.AccountId)
		if err != nil {
			return nil, fmt.Errorf("invalid account ID: %v", err)
		}

		caveats = append(caveats, accounts.CaveatFromID(*id))
		accountID = fn.Some(*id)

	// For the custom macaroon type, we use the custom permissions specified
	// in the request. For the time being, the caveats list will be empty
	// for this type.
	case session.TypeMacaroonCustom:
		if len(req.MacaroonCustomPermissions) == 0 {
			return nil, fmt.Errorf("custom macaroon " +
				"permissions must be specified for the " +
				"custom macaroon session type")
		}

		for _, op := range req.MacaroonCustomPermissions {
			if op.Entity != macaroons.PermissionEntityCustomURI {
				addPerm(op.Entity, op.Action)

				continue
			}

			// If the action specified was equal to the
			// readOnlyAction keyword, then this is taken to mean
			// that the permissions for all read-only URIs should be
			// granted.
			if op.Action == readOnlyAction {
				readPerms := s.cfg.permMgr.ActivePermissions(
					true,
				)

				for _, p := range readPerms {
					addPerm(p.Entity, p.Action)
				}

				continue
			}

			// First check if this is a regex URI.
			uris, isRegex := s.cfg.permMgr.MatchRegexURI(op.Action)
			if isRegex {
				// This is a regex URI, and so we add each of
				// the matching URIs returned from the
				// permissions' manager.
				for _, uri := range uris {
					addPerm(op.Entity, uri)
				}
				continue
			}

			// This is not a wild card URI, so just check that the
			// permissions' manager is aware of this URI.
			_, ok := s.cfg.permMgr.URIPermissions(op.Action)
			if !ok {
				return nil, fmt.Errorf("URI %s is unknown to "+
					"LiT", op.Action)
			}

			addPerm(op.Entity, op.Action)
		}

	// No other types are currently supported.
	default:
		return nil, fmt.Errorf("invalid session type, only admin, " +
			"readonly, custom and account macaroon types supported in " +
			"LiT. Autopilot sessions must be added using " +
			"AddAutoPilotSession method")
	}

	// Collect the de-duped permissions.
	var uniquePermissions []bakery.Op
	for entity, actions := range permissions {
		for action := range actions {
			uniquePermissions = append(uniquePermissions, bakery.Op{
				Entity: entity,
				Action: action,
			})
		}
	}

	sessOpts := []session.Option{
		session.WithMacaroonRecipe(caveats, uniquePermissions),
	}

	if req.DevServer {
		sessOpts = append(sessOpts, session.WithDevServer())
	}

	accountID.WhenSome(func(id accounts.AccountID) {
		sessOpts = append(sessOpts, session.WithAccount(id))
	})

	sess, err := s.cfg.db.NewSession(
		ctx, req.Label, typ, expiry, req.MailboxServerAddr,
		sessOpts...,
	)
	if err != nil {
		return nil, fmt.Errorf("error creating new session: %v", err)
	}

	err = s.cfg.db.ShiftState(ctx, sess.ID, session.StateCreated)
	if err != nil {
		return nil, fmt.Errorf("error shifting session state to "+
			"Created: %v", err)
	}

	if err := s.resumeSession(ctx, sess); err != nil {
		return nil, fmt.Errorf("error starting session: %v", err)
	}

	// Re-fetch the session to get the latest state of it before marshaling
	// it.
	sess, err = s.cfg.db.GetSession(ctx, sess.ID)
	if err != nil {
		return nil, fmt.Errorf("error fetching session: %v", err)
	}

	rpcSession, err := s.marshalRPCSession(ctx, sess)
	if err != nil {
		return nil, fmt.Errorf("error marshaling session: %v", err)
	}

	return &litrpc.AddSessionResponse{
		Session: rpcSession,
	}, nil
}

// resumeSession tries to start the given session if it is not expired.
func (s *sessionRpcServer) resumeSession(ctx context.Context,
	sess *session.Session) error {

	pubKey := sess.LocalPublicKey
	pubKeyBytes := pubKey.SerializeCompressed()

	// Don't resume an expired session.
	if sess.Expiry.Before(time.Now()) {
		log.Debugf("Not resuming session %x with expiry %s",
			pubKeyBytes, sess.Expiry)

		err := s.cfg.db.ShiftState(ctx, sess.ID, session.StateExpired)
		if err != nil {
			return fmt.Errorf("error revoking session: %v", err)
		}

		return nil
	}

	var (
		caveats     []macaroon.Caveat
		permissions []bakery.Op
		readOnly    = sess.Type == session.TypeMacaroonReadonly
	)
	switch sess.Type {
	// For the default session types we use empty caveats and permissions,
	// the macaroons are baked correctly when creating the session.
	case session.TypeMacaroonAdmin, session.TypeMacaroonReadonly:
		permissions = s.cfg.permMgr.ActivePermissions(readOnly)

	// For account based sessions we just add the account ID caveat, the
	// permissions are added dynamically when creating the session.
	case session.TypeMacaroonAccount:
		if sess.MacaroonRecipe == nil {
			return fmt.Errorf("invalid account session, expected " +
				"recipe to be set")
		}

		caveats = sess.MacaroonRecipe.Caveats
		permissions = accounts.MacaroonPermissions

	// For custom session types, we use the caveats and permissions that
	// were persisted on session creation.
	case session.TypeMacaroonCustom, session.TypeAutopilot:
		if sess.MacaroonRecipe == nil {
			break
		}

		permissions = sess.MacaroonRecipe.Permissions
		caveats = append(caveats, sess.MacaroonRecipe.Caveats...)

	// No other types are currently supported.
	default:
		log.Debugf("Not resuming session %x with type %d", pubKeyBytes,
			sess.Type)
		return nil
	}

	// Add the session expiry as a macaroon caveat.
	macExpiry := checkers.TimeBeforeCaveat(sess.Expiry)
	caveats = append(caveats, macaroon.Caveat{
		Id: []byte(macExpiry.Condition),
	})

	mac, err := s.cfg.superMacBaker(
		ctx, sess.MacaroonRootKey, permissions, caveats,
	)
	if err != nil {
		log.Debugf("Not resuming session %x. Could not bake "+
			"the necessary macaroon: %v", pubKeyBytes, err)
		return nil
	}

	var (
		onNewStatus     func(s mailbox.ServerStatus)
		firstConnTimout = make(chan struct{})
	)

	// If this is the first time the session is being spun up then we will
	// kick off a timer to revoke the session after a timeout unless an
	// initial connection is made. We identify such a session as one that
	// we do not yet have a static remote pub key for.
	if sess.RemotePublicKey == nil {
		deadline := sess.CreatedAt.Add(s.cfg.firstConnectionDeadline)
		if deadline.Before(time.Now()) {
			log.Debugf("Deadline for session %x has already "+
				"passed. Revoking session", pubKeyBytes)

			return s.cfg.db.ShiftState(
				ctx, sess.ID, session.StateRevoked,
			)
		}

		// Start the deadline timer.
		deadlineDuration := time.Until(deadline)
		deadlineTimer := time.AfterFunc(deadlineDuration, func() {
			close(firstConnTimout)
		})

		log.Warnf("Kicking off deadline timer for first connection "+
			"for session %x. A successful connection must be "+
			"made in the next %s", pubKeyBytes, deadlineDuration)

		var stopTimerOnce sync.Once
		onNewStatus = func(s mailbox.ServerStatus) {
			// We will only stop the timer if the server status
			// indicates that the client has successfully connected.
			if s != mailbox.ServerStatusInUse {
				return
			}

			// Stop the deadline timer.
			stopTimerOnce.Do(func() {
				log.Debugf("First connection for session %x "+
					"made in a timely manner",
					sess.LocalPublicKey.
						SerializeCompressed())

				deadlineTimer.Stop()
			})
		}
	}

	authData := []byte(fmt.Sprintf("%s: %s", HeaderMacaroon, mac))
	sessionClosedSub, err := s.sessionServer.StartSession(
		sess, authData, s.cfg.db.UpdateSessionRemotePubKey, onNewStatus,
	)
	if err != nil {
		return err
	}

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()

		ticker := time.NewTimer(time.Until(sess.Expiry))
		defer ticker.Stop()

		select {
		case <-s.quit:
			return

		case <-sessionClosedSub:
			return

		case <-ticker.C:
			log.Debugf("Stopping expired session %x with "+
				"type %d", pubKeyBytes, sess.Type)

		case <-firstConnTimout:
			log.Debugf("Deadline exceeded for first connection "+
				"for session %x. Stopping and revoking.",
				pubKeyBytes)
		}

		if s.cfg.autopilot != nil {
			ctxc, cancel := context.WithTimeout(
				ctx, defaultConnectTimeout,
			)

			s.cfg.autopilot.SessionRevoked(ctxc, pubKey)
			cancel()
		}

		err = s.sessionServer.StopSession(pubKey)
		if err != nil {
			log.Debugf("Error stopping session: %v", err)
		}

		err = s.cfg.db.ShiftState(ctx, sess.ID, session.StateRevoked)
		if err != nil {
			log.Debugf("error revoking session: %v", err)
		}
	}()

	return nil
}

// ListSessions returns all sessions known to the session store.
func (s *sessionRpcServer) ListSessions(ctx context.Context,
	_ *litrpc.ListSessionsRequest) (*litrpc.ListSessionsResponse, error) {

	sessions, err := s.cfg.db.ListAllSessions(ctx)
	if err != nil {
		return nil, fmt.Errorf("error fetching sessions: %v", err)
	}

	response := &litrpc.ListSessionsResponse{
		Sessions: make([]*litrpc.Session, len(sessions)),
	}
	for idx, sess := range sessions {
		response.Sessions[idx], err = s.marshalRPCSession(ctx, sess)
		if err != nil {
			return nil, fmt.Errorf("error marshaling session: %v",
				err)
		}
	}

	return response, nil
}

// RevokeSession revokes a single session and also stops it if it is currently
// active.
func (s *sessionRpcServer) RevokeSession(ctx context.Context,
	req *litrpc.RevokeSessionRequest) (*litrpc.RevokeSessionResponse, error) {

	pubKey, err := btcec.ParsePubKey(req.LocalPublicKey)
	if err != nil {
		return nil, fmt.Errorf("error parsing public key: %v", err)
	}

	sess, err := s.cfg.db.GetSessionByLocalPub(ctx, pubKey)
	if err != nil {
		return nil, fmt.Errorf("error fetching session: %v", err)
	}

	err = s.cfg.db.ShiftState(ctx, sess.ID, session.StateRevoked)
	if err != nil {
		return nil, fmt.Errorf("error revoking session: %v", err)
	}

	if s.cfg.autopilot != nil {
		s.cfg.autopilot.SessionRevoked(ctx, pubKey)
	}

	// If the session expired already it might not be running anymore. So we
	// only log possible errors here.
	if err := s.sessionServer.StopSession(pubKey); err != nil {
		log.Debugf("Error stopping session: %v", err)
	}

	return &litrpc.RevokeSessionResponse{}, nil
}

// PrivacyMapConversion can be used map real values to their pseudo counterpart
// and vice versa.
func (s *sessionRpcServer) PrivacyMapConversion(ctx context.Context,
	req *litrpc.PrivacyMapConversionRequest) (
	*litrpc.PrivacyMapConversionResponse, error) {

	var (
		groupID session.ID
		err     error
	)
	if len(req.GroupId) != 0 {
		groupID, err = session.IDFromBytes(req.GroupId)
		if err != nil {
			return nil, err
		}
	} else {
		sessionID, err := session.IDFromBytes(req.SessionId)
		if err != nil {
			return nil, err
		}

		groupID, err = s.cfg.db.GetGroupID(ctx, sessionID)
		if err != nil {
			return nil, err
		}
	}

	var res string
	privMap := s.cfg.privMap.PrivacyDB(groupID)
	err = privMap.View(ctx, func(ctx context.Context,
		tx firewalldb.PrivacyMapTx) error {

		var err error
		if req.RealToPseudo {
			res, err = tx.RealToPseudo(ctx, req.Input)
			return err
		}

		res, err = tx.PseudoToReal(ctx, req.Input)
		return err
	})
	if err != nil {
		return nil, err
	}

	return &litrpc.PrivacyMapConversionResponse{
		Output: res,
	}, nil
}

// ListActions will return a list of actions that have been performed on the
// node. The actions that will be persisted depends on the value of the
// `--firewall.request-logger.level` config option. The default value of the
// option is the "interceptor" mode which will persist only the actions (with
// all request parameters) made with macaroons with caveats that force them to
// be checked by an rpc middleware interceptor. If the "all" mode is used then
// all actions will be persisted but only full request parameters will only be
// stored if the actions are interceptor actions, otherwise only the URI and
// timestamp of the actions will be stored. The "full" mode will persist all
// request data for all actions.
func (s *sessionRpcServer) ListActions(ctx context.Context,
	req *litrpc.ListActionsRequest) (*litrpc.ListActionsResponse, error) {

	// If no maximum number of actions is given, use a default of 100.
	if req.MaxNumActions == 0 {
		req.MaxNumActions = 100
	}

	query := &firewalldb.ListActionsQuery{
		IndexOffset: req.IndexOffset,
		MaxNum:      req.MaxNumActions,
		Reversed:    req.Reversed,
		CountAll:    req.CountTotal,
	}

	state, err := unmarshalActionState(req.State)
	if err != nil {
		return nil, err
	}

	var (
		listOptions = []firewalldb.ListActionOption{
			firewalldb.WithActionFeatureName(req.FeatureName),
			firewalldb.WithActionActorName(req.ActorName),
			firewalldb.WithActionMethodName(req.MethodName),
			firewalldb.WithActionState(state),
		}
		addOption = func(opt firewalldb.ListActionOption) {
			listOptions = append(listOptions, opt)
		}
	)
	if req.SessionId != nil {
		sessionID, err := session.IDFromBytes(req.SessionId)
		if err != nil {
			return nil, err
		}
		addOption(firewalldb.WithActionSessionID(sessionID))
	} else if req.GroupId != nil {
		groupID, err := session.IDFromBytes(req.GroupId)
		if err != nil {
			return nil, err
		}
		addOption(firewalldb.WithActionGroupID(groupID))
	}

	if req.EndTimestamp != 0 {
		addOption(firewalldb.WithActionEndTime(
			time.Unix(int64(req.EndTimestamp), 0)),
		)
	}

	if req.StartTimestamp != 0 {
		addOption(firewalldb.WithActionStartTime(
			time.Unix(int64(req.StartTimestamp), 0)),
		)
	}

	actions, lastIndex, totalCount, err := s.cfg.actionsDB.ListActions(
		ctx, query, listOptions...,
	)
	if err != nil {
		return nil, err
	}

	resp := make([]*litrpc.Action, len(actions))
	for i, a := range actions {
		state, err := marshalActionState(a.State)
		if err != nil {
			return nil, err
		}

		var sessionID session.ID
		a.SessionID.WhenSome(func(id session.ID) {
			sessionID = id
		})

		resp[i] = &litrpc.Action{
			SessionId:          sessionID[:],
			MacaroonIdentifier: a.MacaroonIdentifier[:],
			ActorName:          a.ActorName,
			FeatureName:        a.FeatureName,
			Trigger:            a.Trigger,
			Intent:             a.Intent,
			StructuredJsonData: a.StructuredJsonData,
			RpcMethod:          a.RPCMethod,
			RpcParamsJson:      string(a.RPCParamsJson),
			Timestamp:          uint64(a.AttemptedAt.Unix()),
			State:              state,
			ErrorReason:        a.ErrorReason,
		}
	}

	return &litrpc.ListActionsResponse{
		Actions:         resp,
		LastIndexOffset: lastIndex,
		TotalCount:      totalCount,
	}, nil
}

// ListAutopilotFeatures fetches all the features supported by the autopilot
// server along with the rules that we need to support in order to subscribe
// to those features.
func (s *sessionRpcServer) ListAutopilotFeatures(ctx context.Context,
	_ *litrpc.ListAutopilotFeaturesRequest) (
	*litrpc.ListAutopilotFeaturesResponse, error) {

	fs, err := s.cfg.autopilot.ListFeatures(ctx)
	if err != nil {
		return nil, err
	}

	features := make(map[string]*litrpc.Feature, len(fs))
	for i, f := range fs {
		rules, upgrade, err := convertRules(s.cfg.ruleMgrs, f.Rules)
		if err != nil {
			return nil, err
		}

		// We check whether we should upgrade because of unknown privacy
		// flags.
		_, err = session.Deserialize(f.PrivacyFlags)
		if errors.Is(err, session.ErrUnknownPrivacyFlag) {
			upgrade = true
		} else if err != nil {
			return nil, err
		}

		features[i] = &litrpc.Feature{
			Name:            f.Name,
			Description:     f.Description,
			Rules:           rules,
			PermissionsList: marshalPerms(f.Permissions),
			RequiresUpgrade: upgrade,
			DefaultConfig:   string(f.DefaultConfig),
			PrivacyFlags:    f.PrivacyFlags,
		}
	}

	return &litrpc.ListAutopilotFeaturesResponse{
		Features: features,
	}, nil
}

// AddAutopilotSession creates a new LNC session and attempts to register it
// with the Autopilot server.
func (s *sessionRpcServer) AddAutopilotSession(ctx context.Context,
	req *litrpc.AddAutopilotSessionRequest) (
	*litrpc.AddAutopilotSessionResponse, error) {

	if len(req.Features) == 0 {
		return nil, fmt.Errorf("must include at least one feature")
	}

	expiry := time.Unix(int64(req.ExpiryTimestampSeconds), 0)
	if time.Now().After(expiry) {
		return nil, fmt.Errorf("expiry must be in the future")
	}

	// If the privacy mapper is being used for this session, then we need
	// to keep track of all our known privacy map pairs for this session
	// along with any new pairs that we need to persist.
	var (
		privacy           = !req.NoPrivacyMapper
		knownPrivMapPairs = firewalldb.NewPrivacyMapPairs(nil)
		newPrivMapPairs   = make(map[string]string)
	)

	// If a previous session ID has been set to link this new one to, we
	// first check if we have the referenced session, and we make sure it
	// has been revoked.
	var (
		linkedGroupID      *session.ID
		linkedGroupSession *session.Session
	)
	if len(req.LinkedGroupId) != 0 {
		var groupID session.ID
		copy(groupID[:], req.LinkedGroupId)

		// Check that the group actually does exist.
		groupSess, err := s.cfg.db.GetSession(ctx, groupID)
		if err != nil {
			return nil, err
		}

		// Ensure that the linked session is in fact the first session
		// in its group.
		if groupSess.ID != groupSess.GroupID {
			return nil, fmt.Errorf("can not link to session "+
				"%x since it is not the first in the session "+
				"group %x", groupSess.ID, groupSess.GroupID)
		}

		linkedGroupID = &groupID
		linkedGroupSession = groupSess

		privDB := s.cfg.privMap.PrivacyDB(groupID)
		err = privDB.View(ctx, func(ctx context.Context,
			tx firewalldb.PrivacyMapTx) error {

			knownPrivMapPairs, err = tx.FetchAllPairs(ctx)

			return err
		})
		if err != nil {
			return nil, err
		}
	}

	// First need to fetch all the perms that need to be baked into this
	// mac based on the features.
	allFeatures, err := s.cfg.autopilot.ListFeatures(ctx)
	if err != nil {
		return nil, err
	}

	// Create lookup map of all the features that autopilot server offers.
	autopilotFeatureMap := make(map[string]*autopilotserver.Feature)
	for _, f := range allFeatures {
		autopilotFeatureMap[f.Name] = f
	}

	// allRules represents all the rules that our firewall knows about.
	allRules := s.cfg.ruleMgrs.GetAllRules()

	// Check that each requested feature is a valid autopilot feature and
	// that the necessary rules for the feature have been specified.
	featureRules := make(map[string]map[string]string, len(req.Features))

	// Determine privacy flags to use for session registration.
	var privacyFlags session.PrivacyFlags
	if req.PrivacyFlagsSet {
		if !privacy {
			return nil, fmt.Errorf("privacy flags can only be " +
				"set when the privacy mapper is enabled")
		}

		// We apply privacy flags from the session request in order to
		// to be able to set flags resulting from non-standard feature
		// configurations.
		privacyFlags, err = session.Deserialize(req.PrivacyFlags)
		if err != nil {
			return nil, fmt.Errorf("error deserializing privacy "+
				"flags (%v) from request: %w",
				req.PrivacyFlags, err)
		}
	} else {
		// Otherwise, privacyFlags will contain all the combined (ORed)
		// privacy flags for all requested features with defaults from
		// the autopilot. This means that if any of the features
		// includes a less restrictive privacy flag, this will also
		// apply to features that in principle can be run with better
		// privacy. Checks for features' privacy flag and previous
		// session compatibility are done on the autopilot's side for
		// upgrade flexibility.
		for f := range req.Features {
			// Check that the features is known by the autopilot
			// server.
			autopilotFeature, ok := autopilotFeatureMap[f]
			if !ok {
				return nil, fmt.Errorf("%s is not a features "+
					"provided by the Autopilot server", f)
			}

			// Deserialize and check that we know the privacy flags.
			featurePrivacyFlags, err := session.Deserialize(
				autopilotFeature.PrivacyFlags,
			)
			if err != nil {
				return nil, fmt.Errorf("error deserializing "+
					"privacy flags (%v) from autopilot: %w",
					autopilotFeature.PrivacyFlags, err)
			}

			// We combine all privacy flags.
			privacyFlags = privacyFlags.Add(featurePrivacyFlags)
		}
	}

	for f, rs := range req.Features {
		// Check that the features is known by the autopilot server.
		autopilotFeature, ok := autopilotFeatureMap[f]
		if !ok {
			return nil, fmt.Errorf("%s is not a features "+
				"provided by the Autopilot server", f)
		}

		// reqRules is the rules specified in the request.
		var reqRules []rules.Values
		if rs.Rules != nil {
			reqRules = make([]rules.Values, 0, len(rs.Rules.Rules))
			for ruleName, rule := range rs.Rules.Rules {
				v, err := s.cfg.ruleMgrs.UnmarshalRuleValues(
					ruleName, rule,
				)
				if err != nil {
					return nil, err
				}

				if privacy {
					var privMapPairs map[string]string
					v, privMapPairs, err = v.RealToPseudo(
						ctx, knownPrivMapPairs,
						privacyFlags,
					)
					if err != nil {
						return nil, err
					}

					// Store the new privacy map pairs in
					// the newPrivMap pairs map so that
					// they are later persisted to the real
					// priv map db.
					for k, v := range privMapPairs {
						newPrivMapPairs[k] = v
					}

					// Also add the new pairs to the known
					// set of pairs.
					err = knownPrivMapPairs.Add(
						privMapPairs,
					)
					if err != nil {
						return nil, err
					}
				}

				reqRules = append(reqRules, v)
			}
		}

		// Create a lookup map for the rules specified in this feature.
		// Also check that each of the rules in the request is one known
		// to us.
		frs := make(map[string]rules.Values)
		for _, r := range reqRules {
			frs[r.RuleName()] = r
			_, ok := allRules[r.RuleName()]
			if !ok {
				return nil, fmt.Errorf("%s is not a known rule",
					r.RuleName())
			}
		}

		// For each of the specified rules, we check that the values
		// set for those rules are sane given the bounds provided by
		// the autopilot server for the given feature.
		for _, r := range reqRules {
			ruleName := r.RuleName()

			autopilotSpecs, ok := autopilotFeature.Rules[ruleName]
			if !ok {
				return nil, fmt.Errorf("autopilot did not "+
					"specify %s as a rule for feature %s",
					ruleName, autopilotFeature.Name)
			}

			min, err := s.cfg.ruleMgrs.InitRuleValues(
				ruleName, autopilotSpecs.MinVal,
			)
			if err != nil {
				return nil, err
			}

			max, err := s.cfg.ruleMgrs.InitRuleValues(
				ruleName, autopilotSpecs.MaxVal,
			)
			if err != nil {
				return nil, err
			}

			if err = r.VerifySane(min, max); err != nil {
				return nil, fmt.Errorf("rule value for %s "+
					"not valid for feature %s. Expected "+
					"rule value between %s and %s. Got "+
					"%s. %v", ruleName,
					autopilotFeature.Name, min, max, r, err)
			}
		}

		// If the request did not contain specific values for a rule,
		// the default Autopilot rule value is used.
		finalRules := make(
			[]rules.Values, 0, len(autopilotFeature.Rules),
		)
		for name, values := range autopilotFeature.Rules {
			if r, ok := frs[name]; ok {
				finalRules = append(finalRules, r)
				continue
			}

			defaults, err := s.cfg.ruleMgrs.InitRuleValues(
				name, values.Default,
			)
			if err != nil {
				return nil, err
			}

			finalRules = append(finalRules, defaults)
		}

		featureRules[f], err = marshalRulesToStringMap(finalRules)
		if err != nil {
			return nil, err
		}
	}

	interceptRules := &firewall.InterceptRules{
		FeatureRules: featureRules,
	}

	// Gather all the permissions we need to add to the macaroon given the
	// feature list.
	var dedupedPerms = make(map[string]bool)
	for name, feature := range autopilotFeatureMap {
		if _, ok := req.Features[name]; !ok {
			continue
		}

		for uri := range feature.Permissions {
			dedupedPerms[uri] = true
		}
	}

	var perms []bakery.Op
	for uri := range dedupedPerms {
		perms = append(perms, bakery.Op{
			Entity: macaroons.PermissionEntityCustomURI,
			Action: uri,
		})
	}

	rulesCaveatStr, err := firewall.RulesToCaveat(interceptRules)
	if err != nil {
		return nil, err
	}

	clientConfig := make(session.FeaturesConfig, len(req.Features))
	for name, f := range req.Features {
		clientConfig[name] = f.Config
	}

	caveats := []macaroon.Caveat{{Id: []byte(rulesCaveatStr)}}
	if privacy {
		caveats = append(caveats, firewall.MetaPrivacyCaveat)
	}

	// Construct the functional options that will be used to create the
	// session.
	sessOpts := []session.Option{
		session.WithMacaroonRecipe(caveats, perms),
		session.WithFeatureConfig(clientConfig),
		session.WithLinkedGroupID(linkedGroupID),
	}

	if req.DevServer {
		sessOpts = append(sessOpts, session.WithDevServer())
	}

	if privacy {
		sessOpts = append(sessOpts, session.WithPrivacy(privacyFlags))
	}

	sess, err := s.cfg.db.NewSession(
		ctx, req.Label, session.TypeAutopilot, expiry,
		req.MailboxServerAddr, sessOpts...,
	)
	if err != nil {
		return nil, fmt.Errorf("error creating new session: %v", err)
	}

	// If this session is being linked to a previous one, then we need to
	// use the previous session's local private key to sign the new
	// session's public key in order to prove to the Autopilot server that
	// the two session's belong to the same owner.
	var (
		linkSig        []byte
		prevSessionPub *btcec.PublicKey
	)
	if linkedGroupID != nil {
		privKey := linkedGroupSession.LocalPrivateKey
		pubKey := sess.LocalPublicKey.SerializeCompressed()

		msg := chainhash.HashB(pubKey)
		linkSig = ecdsa.Sign(privKey, msg).Serialize()

		prevSessionPub = linkedGroupSession.LocalPublicKey
	}

	// The feature configurations may contain sensitive data like pubkeys,
	// which we replace here and add to the privacy map.
	obfuscatedConfig := make(session.FeaturesConfig, len(clientConfig))
	if privacy {
		for name, configB := range clientConfig {
			configB, privMapPairs, err := firewall.ObfuscateConfig(
				knownPrivMapPairs, configB, privacyFlags,
			)
			if err != nil {
				return nil, err
			}

			// Store the new privacy map pairs in the newPrivMap
			// pairs map so that they are later persisted to the
			// real priv map db.
			for k, v := range privMapPairs {
				newPrivMapPairs[k] = v
			}

			// Also add the new pairs to the known set of pairs.
			err = knownPrivMapPairs.Add(privMapPairs)
			if err != nil {
				return nil, err
			}

			obfuscatedConfig[name] = configB
		}
	} else {
		obfuscatedConfig = clientConfig
	}

	// Register all the privacy map pairs for this session ID.
	privDB := s.cfg.privMap.PrivacyDB(sess.GroupID)
	err = privDB.Update(ctx, func(ctx context.Context,
		tx firewalldb.PrivacyMapTx) error {

		for r, p := range newPrivMapPairs {
			err := tx.NewPair(ctx, r, p)
			if err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	// Attempt to register the session with the Autopilot server.
	remoteKey, err := s.cfg.autopilot.RegisterSession(
		ctx, sess.LocalPublicKey, sess.ServerAddr, sess.DevServer,
		obfuscatedConfig, prevSessionPub, linkSig,
		privacyFlags.Serialize(),
	)
	if err != nil {
		return nil, fmt.Errorf("error registering session with "+
			"autopilot server: %v", err)
	}

	err = s.cfg.db.UpdateSessionRemotePubKey(ctx, sess.ID, remoteKey)
	if err != nil {
		return nil, fmt.Errorf("error setting remote pubkey: %v", err)
	}

	// Update our in-memory session with the remote key.
	sess.RemotePublicKey = remoteKey

	// We only activate the session if the Autopilot server registration
	// was successful.
	err = s.cfg.db.ShiftState(ctx, sess.ID, session.StateCreated)
	if err != nil {
		return nil, fmt.Errorf("error shifting session state to "+
			"Created: %v", err)
	}

	if err := s.resumeSession(ctx, sess); err != nil {
		return nil, fmt.Errorf("error starting session: %v", err)
	}

	// Re-fetch the session to get the latest state of it before marshaling
	// it.
	sess, err = s.cfg.db.GetSession(ctx, sess.ID)
	if err != nil {
		return nil, fmt.Errorf("error fetching session: %v", err)
	}

	rpcSession, err := s.marshalRPCSession(ctx, sess)
	if err != nil {
		return nil, fmt.Errorf("error marshaling session: %v", err)
	}

	return &litrpc.AddAutopilotSessionResponse{
		Session: rpcSession,
	}, nil
}

// ListAutopilotSessions fetches and returns all the sessions from the DB that
// are of type TypeAutopilot.
func (s *sessionRpcServer) ListAutopilotSessions(ctx context.Context,
	_ *litrpc.ListAutopilotSessionsRequest) (
	*litrpc.ListAutopilotSessionsResponse, error) {

	sessions, err := s.cfg.db.ListSessionsByType(ctx, session.TypeAutopilot)
	if err != nil {
		return nil, fmt.Errorf("error fetching sessions: %v", err)
	}

	response := &litrpc.ListAutopilotSessionsResponse{
		Sessions: make([]*litrpc.Session, len(sessions)),
	}
	for idx, sess := range sessions {
		response.Sessions[idx], err = s.marshalRPCSession(ctx, sess)
		if err != nil {
			return nil, fmt.Errorf("error marshaling session: %v",
				err)
		}
	}

	return response, nil
}

// RevokeAutopilotSession revokes an autopilot session.
func (s *sessionRpcServer) RevokeAutopilotSession(ctx context.Context,
	req *litrpc.RevokeAutopilotSessionRequest) (
	*litrpc.RevokeAutopilotSessionResponse, error) {

	pubKey, err := btcec.ParsePubKey(req.LocalPublicKey)
	if err != nil {
		return nil, fmt.Errorf("error parsing public key: %v", err)
	}

	sess, err := s.cfg.db.GetSessionByLocalPub(ctx, pubKey)
	if err != nil {
		return nil, err
	}

	if sess.Type != session.TypeAutopilot {
		return nil, session.ErrSessionNotFound
	}

	_, err = s.RevokeSession(
		ctx, &litrpc.RevokeSessionRequest{
			LocalPublicKey: req.LocalPublicKey,
		},
	)
	if err != nil {
		return nil, err
	}

	return &litrpc.RevokeAutopilotSessionResponse{}, nil
}

func marshalRulesToStringMap(rs []rules.Values) (map[string]string, error) {
	res := make(map[string]string, len(rs))
	for _, r := range rs {
		b, err := json.Marshal(r)
		if err != nil {
			return nil, err
		}

		res[r.RuleName()] = string(b)
	}

	return res, nil
}

// marshalPerms attempts to convert a set of permissions into their RPC
// counterpart. If the list includes a rule that LiT does not know about, a nil
// entry is included in the returned map. A bool is returned that indicates if
// an upgrade is needed in order to enforce an unknown rule.
func convertRules(ruleMgr rules.ManagerSet,
	ruleList map[string]*autopilotserver.RuleValues) (
	map[string]*litrpc.RuleValues, bool, error) {

	var (
		upgrade bool
		res     = make(
			map[string]*litrpc.RuleValues, len(ruleList),
		)
		knownRules = ruleMgr.GetAllRules()
	)
	for name, rule := range ruleList {
		if !knownRules[name] {
			upgrade = true
			res[name] = &litrpc.RuleValues{
				Known: false,
			}

			continue
		}

		defaultVals, err := ruleMgr.InitRuleValues(name, rule.Default)
		if err != nil {
			return nil, false, err
		}

		minVals, err := ruleMgr.InitRuleValues(name, rule.MinVal)
		if err != nil {
			return nil, false, err
		}

		maxVals, err := ruleMgr.InitRuleValues(name, rule.MaxVal)
		if err != nil {
			return nil, false, err
		}

		res[name] = &litrpc.RuleValues{
			Known:    true,
			Defaults: defaultVals.ToProto(),
			MinValue: minVals.ToProto(),
			MaxValue: maxVals.ToProto(),
		}
	}

	return res, upgrade, nil
}

// marshalPerms converts a set of permissions into their RPC counterpart.
func marshalPerms(perms map[string][]bakery.Op) []*litrpc.Permissions {
	var res []*litrpc.Permissions

	for method, ops := range perms {
		rpcOps := make([]*litrpc.MacaroonPermission, len(ops))
		for i, op := range ops {
			rpcOps[i] = &litrpc.MacaroonPermission{
				Entity: op.Entity,
				Action: op.Action,
			}
		}

		res = append(res, &litrpc.Permissions{
			Method:     method,
			Operations: rpcOps,
		})
	}

	return res
}

// marshalRPCSession converts a session into its RPC counterpart.
func (s *sessionRpcServer) marshalRPCSession(ctx context.Context,
	sess *session.Session) (*litrpc.Session, error) {

	rpcState, err := marshalRPCState(sess.State)
	if err != nil {
		return nil, err
	}

	rpcType, err := marshalRPCType(sess.Type)
	if err != nil {
		return nil, err
	}

	var remotePubKey []byte
	if sess.RemotePublicKey != nil {
		remotePubKey = sess.RemotePublicKey.SerializeCompressed()
	}

	mnemonic, err := mailbox.PassphraseEntropyToMnemonic(sess.PairingSecret)
	if err != nil {
		return nil, err
	}

	macRecipe := marshalRPCMacaroonRecipe(sess.MacaroonRecipe)

	var revokedAt uint64
	if !sess.RevokedAt.IsZero() {
		revokedAt = uint64(sess.RevokedAt.Unix())
	}

	var (
		featureInfo    = make(map[string]*litrpc.RulesMap)
		initRuleValues = s.cfg.ruleMgrs.InitRuleValues
	)
	if sess.MacaroonRecipe != nil {
		for _, cav := range sess.MacaroonRecipe.Caveats {
			info, err := firewall.ParseRuleCaveat(string(cav.Id))
			if errors.Is(err, firewall.ErrNoRulesCaveat) {
				continue
			} else if err != nil {
				return nil, err
			}

			for feature, rules := range info.FeatureRules {
				ruleMap := make(map[string]*litrpc.RuleValue)
				for name, rule := range rules {
					val, err := initRuleValues(
						name, []byte(rule),
					)
					if err != nil {
						return nil, err
					}

					if sess.WithPrivacyMapper {
						db := s.cfg.privMap.PrivacyDB(
							sess.GroupID,
						)
						val, err = val.PseudoToReal(
							ctx, db,
							sess.PrivacyFlags,
						)
						if err != nil {
							return nil, err
						}
					}

					ruleMap[name] = val.ToProto()
				}

				featureInfo[feature] = &litrpc.RulesMap{
					Rules: ruleMap,
				}
			}
		}
	}

	clientConfig := make(map[string]string)
	if sess.FeatureConfig != nil {
		for k, v := range *sess.FeatureConfig {
			clientConfig[k] = string(v)
		}
	}

	var accountID string
	sess.AccountID.WhenSome(func(id accounts.AccountID) {
		accountID = hex.EncodeToString(id[:])
	})

	return &litrpc.Session{
		Id:                     sess.ID[:],
		Label:                  sess.Label,
		SessionState:           rpcState,
		SessionType:            rpcType,
		ExpiryTimestampSeconds: uint64(sess.Expiry.Unix()),
		MailboxServerAddr:      sess.ServerAddr,
		DevServer:              sess.DevServer,
		PairingSecret:          sess.PairingSecret[:],
		PairingSecretMnemonic:  strings.Join(mnemonic[:], " "),
		LocalPublicKey:         sess.LocalPublicKey.SerializeCompressed(),
		RemotePublicKey:        remotePubKey,
		CreatedAt:              uint64(sess.CreatedAt.Unix()),
		RevokedAt:              revokedAt,
		MacaroonRecipe:         macRecipe,
		AutopilotFeatureInfo:   featureInfo,
		GroupId:                sess.GroupID[:],
		FeatureConfigs:         clientConfig,
		PrivacyFlags:           sess.PrivacyFlags.Serialize(),
		AccountId:              accountID,
	}, nil
}

// marshalRPCMacaroonRecipe converts a macaroon recipe (permissions and caveats)
// into its RPC counterpart.
func marshalRPCMacaroonRecipe(
	recipe *session.MacaroonRecipe) *litrpc.MacaroonRecipe {

	if recipe == nil {
		return nil
	}

	perms := make([]*litrpc.MacaroonPermission, len(recipe.Permissions))
	for i, op := range recipe.Permissions {
		perms[i] = &litrpc.MacaroonPermission{
			Entity: op.Entity,
			Action: op.Action,
		}
	}

	caveats := make([]string, len(recipe.Caveats))
	for i, cav := range recipe.Caveats {
		caveats[i] = string(cav.Id)
	}

	return &litrpc.MacaroonRecipe{
		Permissions: perms,
		Caveats:     caveats,
	}
}

// marshalRPCState converts a session state to its RPC counterpart.
func marshalRPCState(state session.State) (litrpc.SessionState, error) {
	switch state {
	case session.StateReserved:
		return litrpc.SessionState_STATE_RESERVED, nil

	case session.StateCreated:
		return litrpc.SessionState_STATE_CREATED, nil

	case session.StateInUse:
		return litrpc.SessionState_STATE_IN_USE, nil

	case session.StateRevoked:
		return litrpc.SessionState_STATE_REVOKED, nil

	case session.StateExpired:
		return litrpc.SessionState_STATE_EXPIRED, nil

	default:
		return 0, fmt.Errorf("unknown state <%d>", state)
	}
}

// marshalRPCType converts a session type to its RPC counterpart.
func marshalRPCType(typ session.Type) (litrpc.SessionType, error) {
	switch typ {
	case session.TypeMacaroonReadonly:
		return litrpc.SessionType_TYPE_MACAROON_READONLY, nil

	case session.TypeMacaroonAdmin:
		return litrpc.SessionType_TYPE_MACAROON_ADMIN, nil

	case session.TypeMacaroonCustom:
		return litrpc.SessionType_TYPE_MACAROON_CUSTOM, nil

	case session.TypeUIPassword:
		return litrpc.SessionType_TYPE_UI_PASSWORD, nil

	case session.TypeMacaroonAccount:
		return litrpc.SessionType_TYPE_MACAROON_ACCOUNT, nil

	case session.TypeAutopilot:
		return litrpc.SessionType_TYPE_AUTOPILOT, nil

	default:
		return 0, fmt.Errorf("unknown type <%d>", typ)
	}
}

// unmarshalRPCType converts an RPC session type to its session counterpart.
func unmarshalRPCType(typ litrpc.SessionType) (session.Type, error) {
	switch typ {
	case litrpc.SessionType_TYPE_MACAROON_READONLY:
		return session.TypeMacaroonReadonly, nil

	case litrpc.SessionType_TYPE_MACAROON_ADMIN:
		return session.TypeMacaroonAdmin, nil

	case litrpc.SessionType_TYPE_MACAROON_CUSTOM:
		return session.TypeMacaroonCustom, nil

	case litrpc.SessionType_TYPE_UI_PASSWORD:
		return session.TypeUIPassword, nil

	case litrpc.SessionType_TYPE_MACAROON_ACCOUNT:
		return session.TypeMacaroonAccount, nil

	case litrpc.SessionType_TYPE_AUTOPILOT:
		return session.TypeAutopilot, nil

	default:
		return 0, fmt.Errorf("unknown type <%d>", typ)
	}
}

// marshalActionState converts an Action state into its RPC counterpart.
func marshalActionState(state firewalldb.ActionState) (litrpc.ActionState,
	error) {

	switch state {
	case firewalldb.ActionStateUnknown:
		return litrpc.ActionState_STATE_UNKNOWN, nil
	case firewalldb.ActionStateInit:
		return litrpc.ActionState_STATE_PENDING, nil
	case firewalldb.ActionStateDone:
		return litrpc.ActionState_STATE_DONE, nil
	case firewalldb.ActionStateError:
		return litrpc.ActionState_STATE_ERROR, nil
	default:
		return 0, fmt.Errorf("unknown state <%d>", state)
	}
}

func unmarshalActionState(state litrpc.ActionState) (firewalldb.ActionState,
	error) {

	switch state {
	case litrpc.ActionState_STATE_UNKNOWN:
		return firewalldb.ActionStateUnknown, nil
	case litrpc.ActionState_STATE_PENDING:
		return firewalldb.ActionStateInit, nil
	case litrpc.ActionState_STATE_DONE:
		return firewalldb.ActionStateDone, nil
	case litrpc.ActionState_STATE_ERROR:
		return firewalldb.ActionStateError, nil
	default:
		return 0, fmt.Errorf("unknown state <%d>", state)
	}
}
