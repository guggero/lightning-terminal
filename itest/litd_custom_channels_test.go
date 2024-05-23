package itest

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/davecgh/go-spew/spew"
	"github.com/lightninglabs/taproot-assets/itest"
	"github.com/lightninglabs/taproot-assets/proof"
	"github.com/lightninglabs/taproot-assets/rfqmsg"
	"github.com/lightninglabs/taproot-assets/tapchannel"
	"github.com/lightninglabs/taproot-assets/taprpc"
	"github.com/lightninglabs/taproot-assets/taprpc/assetwalletrpc"
	"github.com/lightninglabs/taproot-assets/taprpc/mintrpc"
	"github.com/lightninglabs/taproot-assets/taprpc/rfqrpc"
	tchrpc "github.com/lightninglabs/taproot-assets/taprpc/tapchannelrpc"
	"github.com/lightninglabs/taproot-assets/taprpc/tapdevrpc"
	"github.com/lightninglabs/taproot-assets/taprpc/universerpc"
	"github.com/lightningnetwork/lnd/lnrpc"
	"github.com/lightningnetwork/lnd/lnrpc/routerrpc"
	"github.com/lightningnetwork/lnd/lntest"
	"github.com/lightningnetwork/lnd/lntest/rpc"
	"github.com/lightningnetwork/lnd/lntest/wait"
	"github.com/lightningnetwork/lnd/lntypes"
	"github.com/lightningnetwork/lnd/lnwire"
	"github.com/lightningnetwork/lnd/macaroons"
	"github.com/lightningnetwork/lnd/record"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/protobuf/proto"
	"gopkg.in/macaroon.v2"
)

var (
	dummyMetaData = &taprpc.AssetMeta{
		Data: []byte("some metadata"),
	}

	itestAsset = &mintrpc.MintAsset{
		AssetType: taprpc.AssetType_NORMAL,
		Name:      "itest-asset-cents",
		AssetMeta: dummyMetaData,
		Amount:    500_000_000,
	}

	shortTimeout = time.Second * 5
)

// testCustomChannels tests that we can create a network with custom channels
// and send asset payments over them.
func testCustomChannels(_ context.Context, net *NetworkHarness,
	t *harnessTest) {

	ctxb := context.Background()
	lndArgs := []string{
		"--trickledelay=50",
		"--gossip.sub-batch-delay=5ms",
		"--caches.rpc-graph-cache-duration=100ms",
		"--default-remote-max-htlcs=483",
		"--dust-threshold=5000000",
		"--rpcmiddleware.enable",
		"--protocol.anchors",
		"--protocol.option-scid-alias",
		"--protocol.zero-conf",
		"--protocol.simple-taproot-chans",
		"--protocol.custom-message=17",
		"--accept-keysend",
		"--debuglevel=trace,GRPC=error,BTCN=info",
	}
	litdArgs := []string{
		"--taproot-assets.allow-public-uni-proof-courier",
		"--taproot-assets.universe.public-access",
		"--taproot-assets.universerpccourier.skipinitdelay",
		"--taproot-assets.universerpccourier.backoffresetwait=1s",
		"--taproot-assets.universerpccourier.numtries=5",
		"--taproot-assets.universerpccourier.initialbackoff=300ms",
		"--taproot-assets.universerpccourier.maxbackoff=600ms",
	}

	// Explicitly set the proof courier as Alice (how has no other role
	// other than proof shuffling), otherwise a hashmail courier will be
	// used. For the funding transaction, we're just posting it and don't
	// expect a true receiver.
	zane, err := net.NewNode(
		t.t, "Zane", lndArgs, false, true, litdArgs...,
	)
	require.NoError(t.t, err)

	litdArgs = append(litdArgs, fmt.Sprintf(
		"--taproot-assets.proofcourieraddr=%s://%s",
		proof.UniverseRpcCourierType, zane.Cfg.LitAddr(),
	))

	// The topology we are going for looks like the following:
	// Charlie ---CC---> Dave ---NC---> Erin ---CC---> Fabia
	// With CC being a custom channel and NC being a normal channel.
	// All 4 nodes need to be full litd nodes running in integrated mode
	// with tapd included. We also need specific flags to be enabled, so we
	// create 4 completely new nodes, ignoring the two default nodes that
	// are created by the harness.
	charlie, err := net.NewNode(
		t.t, "Charlie", lndArgs, false, true, litdArgs...,
	)
	require.NoError(t.t, err)

	dave, err := net.NewNode(t.t, "Dave", lndArgs, false, true, litdArgs...)
	require.NoError(t.t, err)
	erin, err := net.NewNode(t.t, "Erin", lndArgs, false, true, litdArgs...)
	require.NoError(t.t, err)
	fabia, err := net.NewNode(
		t.t, "Fabia", lndArgs, false, true, litdArgs...,
	)
	require.NoError(t.t, err)

	nodes := []*HarnessNode{charlie, dave, erin, fabia}
	connectAllNodes(t.t, net, nodes)
	fundAllNodes(t.t, net, nodes)

	// Create the normal channel between Dave and Erin.
	t.Logf("Opening normal channel between Dave and Erin...")
	channelOp := openChannelAndAssert(
		t, net, dave, erin, lntest.OpenChannelParams{
			Amt:         5_000_000,
			SatPerVByte: 5,
		},
	)
	defer closeChannelAndAssert(t, net, dave, channelOp, false)

	universeTap := newTapClient(t.t, zane)
	charlieTap := newTapClient(t.t, charlie)
	daveTap := newTapClient(t.t, dave)
	erinTap := newTapClient(t.t, erin)
	fabiaTap := newTapClient(t.t, fabia)

	// Mint an asset on Charlie and sync all nodes to Charlie as the
	// universe.
	mintedAssets := itest.MintAssetsConfirmBatch(
		t.t, t.lndHarness.Miner.Client, charlieTap,
		[]*mintrpc.MintAssetRequest{
			{
				Asset: itestAsset,
			},
		},
	)
	cents := mintedAssets[0]
	assetID := cents.AssetGenesis.AssetId
	fundingScriptTree := tapchannel.NewFundingScriptTree()
	fundingScriptKey := fundingScriptTree.TaprootKey
	fundingScriptTreeBytes := fundingScriptKey.SerializeCompressed()

	t.Logf("Minted %d lightning cents, syncing universes...", cents.Amount)
	syncUniverses(t.t, charlieTap, dave, erin, fabia)
	t.Logf("Universes synced between all nodes, distributing assets...")

	// We need to send some assets to Erin, so he can fund an asset channel
	// with Fabia.
	const (
		fundingAmount = 50_000
		startAmount   = fundingAmount * 2
	)
	erinAddr, err := erinTap.NewAddr(ctxb, &taprpc.NewAddrRequest{
		Amt:     startAmount,
		AssetId: assetID,
		ProofCourierAddr: fmt.Sprintf(
			"%s://%s", proof.UniverseRpcCourierType,
			charlie.Cfg.LitAddr(),
		),
	})
	require.NoError(t.t, err)

	t.Logf("Sending %v asset units to Erin...", startAmount)

	// Send the assets to Erin.
	itest.AssertAddrCreated(t.t, erinTap, cents, erinAddr)
	sendResp, err := charlieTap.SendAsset(ctxb, &taprpc.SendAssetRequest{
		TapAddrs: []string{erinAddr.Encoded},
	})
	require.NoError(t.t, err)
	itest.ConfirmAndAssertOutboundTransfer(
		t.t, t.lndHarness.Miner.Client, charlieTap, sendResp, assetID,
		[]uint64{cents.Amount - startAmount, startAmount}, 0, 1,
	)
	itest.AssertNonInteractiveRecvComplete(t.t, erinTap, 1)

	// This is the only public channel, we need everyone to be aware of it.
	assertChannelKnown(t.t, charlie, channelOp)

	t.Logf("Opening asset channels...")
	fundResp, err := charlieTap.FundChannel(
		ctxb, &tchrpc.FundChannelRequest{
			AssetAmount:        fundingAmount,
			AssetId:            assetID,
			PeerPubkey:         dave.PubKey[:],
			FeeRateSatPerVbyte: 5,
		},
	)
	require.NoError(t.t, err)
	t.Logf("Funded channel between Charlie and Dave: %v", fundResp)

	fundResp2, err := erinTap.FundChannel(
		ctxb, &tchrpc.FundChannelRequest{
			AssetAmount:        fundingAmount,
			AssetId:            assetID,
			PeerPubkey:         fabia.PubKey[:],
			FeeRateSatPerVbyte: 5,
		},
	)
	require.NoError(t.t, err)
	t.Logf("Funded channel between Erin and Fabia: %v", fundResp2)

	mineBlocks(t, net, 6, 2)

	// We'll be tracking the expected asset balances throughout the test, so
	// we can assert it after each action.
	charlieAssetBalance := cents.Amount - startAmount
	daveAssetBalance := uint64(0)
	erinAssetBalance := uint64(startAmount)
	fabiaAssetBalance := uint64(0)

	// After opening the channels, the asset balance of the funding nodes
	// shouldn't have been decreased, since the asset with the funding
	// output was imported into the asset DB and should count toward the
	// balance.
	assertAssetBalance(t.t, charlieTap, assetID, charlieAssetBalance)
	assertAssetBalance(t.t, erinTap, assetID, erinAssetBalance)

	// We should also find three asset pieces in the asset DB of Charlie,
	// one for the change from the initial send to Erin, one for the change
	// of the funding (for both of which we don't know the script key) and
	// the funding transaction itself.
	assertAssetExists(
		t.t, charlieTap, assetID,
		cents.Amount-startAmount-fundingAmount, nil, true, false, false,
	)
	assertAssetExists(
		t.t, charlieTap, assetID, fundingAmount, fundingScriptKey,
		false, true, true,
	)

	// Erin should just have two equally sized asset pieces, the change and
	// the funding transaction.
	assertAssetExists(
		t.t, erinTap, assetID, startAmount-fundingAmount, nil, true,
		false, false,
	)
	assertAssetExists(
		t.t, erinTap, assetID, fundingAmount, fundingScriptKey,
		false, true, true,
	)

	// Assert that the proofs for both channels has been uploaded to the
	// designated Universe server.
	assertUniverseProofExists(
		t.t, universeTap, assetID, fundingScriptTreeBytes, fmt.Sprintf(
			"%v:%v", fundResp.Txid, fundResp.OutputIndex,
		),
	)
	assertUniverseProofExists(
		t.t, universeTap, assetID, fundingScriptTreeBytes, fmt.Sprintf(
			"%v:%v", fundResp2.Txid, fundResp2.OutputIndex,
		),
	)

	// Make sure the channel shows the correct asset information.
	assertAssetChan(t.t, charlie, dave, fundingAmount, assetID)
	assertAssetChan(t.t, erin, fabia, fundingAmount, assetID)

	// Print initial channel balances.
	balanceCharlie, err := getChannelCustomData(charlie, dave)
	require.NoError(t.t, err)
	t.Logf("Charlie initial balance: %v", toJSON(t.t, balanceCharlie))
	balanceErin, err := getChannelCustomData(erin, fabia)
	require.NoError(t.t, err)
	t.Logf("Erin initial balance: %v", toJSON(t.t, balanceErin))

	// ------------
	// Test case 1: Send a direct keysend payment from Charlie to Dave.
	// ------------
	const keySendAmount = 100
	sendKeySendPayment(t.t, charlie, dave, keySendAmount, assetID)
	balanceCharlie, err = getChannelCustomData(charlie, dave)
	require.NoError(t.t, err)
	t.Logf("Charlie balance after keysend payment: %v",
		toJSON(t.t, balanceCharlie))
	balanceDave, err := getChannelCustomData(dave, charlie)
	require.NoError(t.t, err)
	t.Logf("Dave balance after keysend payment: %v",
		toJSON(t.t, balanceDave))

	charlieAssetBalance -= keySendAmount
	daveAssetBalance += keySendAmount

	// ------------
	// Test case 2: Pay a normal invoice from Erin by Charlie.
	// ------------
	paidAssetAmount := createAndPayNormalInvoice(
		t.t, charlie, dave, erin, 20_000, assetID,
	)
	balanceCharlie, err = getChannelCustomData(charlie, dave)
	require.NoError(t.t, err)
	t.Logf("Charlie balance after invoice payment: %v",
		toJSON(t.t, balanceCharlie))

	charlieAssetBalance -= paidAssetAmount
	daveAssetBalance += paidAssetAmount

	// ------------
	// Test case 3: Create an asset invoice on Fabia and pay it from
	// Charlie.
	// ------------
	const fabiaInvoiceAssetAmount1 = 1000
	invoiceResp := createAssetInvoice(
		t.t, erin, fabia, fabiaInvoiceAssetAmount1, assetID,
	)
	payInvoiceWithAssets(t.t, charlie, dave, invoiceResp, assetID)
	balanceCharlie, err = getChannelCustomData(charlie, dave)
	require.NoError(t.t, err)
	t.Logf("Charlie balance after invoice payment: %v",
		toJSON(t.t, balanceCharlie))
	balanceFabia, err := getChannelCustomData(fabia, erin)
	require.NoError(t.t, err)
	t.Logf("Fabia balance after invoice payment: %v",
		toJSON(t.t, balanceFabia))

	charlieAssetBalance -= fabiaInvoiceAssetAmount1
	daveAssetBalance += fabiaInvoiceAssetAmount1
	erinAssetBalance -= fabiaInvoiceAssetAmount1
	fabiaAssetBalance += fabiaInvoiceAssetAmount1

	// ------------
	// Test case 4: Create an asset invoice on Fabian and pay it with just
	// BTC from Dave, making sure it ends up being a multipart payment.
	// ------------
	const fabiaInvoiceAssetAmount2 = 15_000
	invoiceResp = createAssetInvoice(
		t.t, erin, fabia, fabiaInvoiceAssetAmount2, assetID,
	)
	payInvoiceWithSatoshi(t.t, dave, invoiceResp)
	balanceFabia, err = getChannelCustomData(fabia, erin)
	require.NoError(t.t, err)
	t.Logf("Fabia balance after invoice payment: %v",
		toJSON(t.t, balanceFabia))

	erinAssetBalance -= fabiaInvoiceAssetAmount2
	fabiaAssetBalance += fabiaInvoiceAssetAmount2

	// ------------
	// Test case 5: Create an asset invoice on Fabian and pay it with assets
	// from Charlie, making sure it ends up being a multipart payment as
	// well.
	// ------------
	const fabiaInvoiceAssetAmount3 = 10_000
	invoiceResp = createAssetInvoice(
		t.t, erin, fabia, fabiaInvoiceAssetAmount3, assetID,
	)
	payInvoiceWithAssets(t.t, charlie, dave, invoiceResp, assetID)
	balanceFabia, err = getChannelCustomData(fabia, erin)
	require.NoError(t.t, err)
	t.Logf("Fabia balance after invoice payment: %v",
		toJSON(t.t, balanceFabia))

	charlieAssetBalance -= fabiaInvoiceAssetAmount3
	daveAssetBalance += fabiaInvoiceAssetAmount3
	erinAssetBalance -= fabiaInvoiceAssetAmount3
	fabiaAssetBalance += fabiaInvoiceAssetAmount3

	// ------------
	// Test case 6: Now we'll close each of the channels, starting with the
	// Charlie -> Dave custom channel.
	//
	// First, we'll grab the current asset balance for Charlie + dave.
	// ------------
	balanceCharlie, err = getChannelCustomData(charlie, dave)
	require.NoError(t.t, err)

	// With the balances known, we'll now issue a normal co-op close
	// channel request.
	charlieChanPoint := &lnrpc.ChannelPoint{
		OutputIndex: uint32(fundResp.OutputIndex),
		FundingTxid: &lnrpc.ChannelPoint_FundingTxidStr{
			FundingTxidStr: fundResp.Txid,
		},
	}
	erinChanPoint := &lnrpc.ChannelPoint{
		OutputIndex: uint32(fundResp2.OutputIndex),
		FundingTxid: &lnrpc.ChannelPoint_FundingTxidStr{
			FundingTxidStr: fundResp2.Txid,
		},
	}

	t.Logf("Closing Charlie -> Dave channel")
	closeAssetChannelAndAssert(
		t, net, charlie, charlieChanPoint, assetID, universeTap, true,
		true,
	)

	t.Logf("Closing Erin -> Fabia channel")
	closeAssetChannelAndAssert(
		t, net, erin, erinChanPoint, assetID, universeTap, true, true,
	)

	// We've been tracking the off-chain channel balances all this time, so
	// now that we have the assets on-chain again, we can assert them. Due
	// to rounding errors that happened when sending multiple shards with
	// MPP, we need to do some slight adjustments.
	charlieAssetBalance += 2
	erinAssetBalance += 4
	fabiaAssetBalance -= 2
	assertAssetBalance(t.t, charlieTap, assetID, charlieAssetBalance)
	assertAssetBalance(t.t, daveTap, assetID, daveAssetBalance)
	assertAssetBalance(t.t, erinTap, assetID, erinAssetBalance)
	assertAssetBalance(t.t, fabiaTap, assetID, fabiaAssetBalance)

	// ------------
	// Test case 7: We now open a new asset channel and close it again, to
	// make sure that a non-existent remote balance is handled correctly.
	t.Logf("Opening new asset channel...")
	fundResp, err = charlieTap.FundChannel(
		ctxb, &tchrpc.FundChannelRequest{
			AssetAmount:        fundingAmount,
			AssetId:            assetID,
			PeerPubkey:         dave.PubKey[:],
			FeeRateSatPerVbyte: 5,
		},
	)
	require.NoError(t.t, err)
	t.Logf("Funded second channel between Charlie and Dave: %v", fundResp)

	mineBlocks(t, net, 6, 1)

	// Assert that the proofs for both channels has been uploaded to the
	// designated Universe server.
	assertUniverseProofExists(
		t.t, universeTap, assetID, fundingScriptTreeBytes, fmt.Sprintf(
			"%v:%v", fundResp.Txid, fundResp.OutputIndex,
		),
	)
	assertAssetChan(t.t, charlie, dave, fundingAmount, assetID)

	// And let's just close the channel again.
	charlieChanPoint = &lnrpc.ChannelPoint{
		OutputIndex: uint32(fundResp.OutputIndex),
		FundingTxid: &lnrpc.ChannelPoint_FundingTxidStr{
			FundingTxidStr: fundResp.Txid,
		},
	}
	closeAssetChannelAndAssert(
		t, net, charlie, charlieChanPoint, assetID, universeTap, false,
		false,
	)

	// TODO(guggero): We're double counting two funding outputs.
	charlieAssetBalance += fundingAmount

	// Charlie should still have four asset pieces, two with the same size.
	assertAssetExists(
		t.t, charlieTap, assetID, cents.Amount-2*startAmount, nil, true,
		false, false,
	)
	assertAssetExists(
		t.t, charlieTap, assetID, fundingAmount, nil, false, true,
		true,
	)
	assertAssetExists(
		t.t, charlieTap, assetID, fundingAmount, nil, true, true,
		false,
	)
	remainder := charlieAssetBalance - (cents.Amount - 2*startAmount) -
		2*fundingAmount
	assertAssetExists(
		t.t, charlieTap, assetID, remainder, nil, true, true, false,
	)

	// The asset balances should still remain unchanged.
	assertAssetBalance(t.t, charlieTap, assetID, charlieAssetBalance)
	assertAssetBalance(t.t, daveTap, assetID, daveAssetBalance)
	assertAssetBalance(t.t, erinTap, assetID, erinAssetBalance)
	assertAssetBalance(t.t, fabiaTap, assetID, fabiaAssetBalance)
}

func connectAllNodes(t *testing.T, net *NetworkHarness, nodes []*HarnessNode) {
	for i, node := range nodes {
		for j := i + 1; j < len(nodes); j++ {
			peer := nodes[j]
			net.ConnectNodesPerm(t, node, peer)
		}
	}
}

func fundAllNodes(t *testing.T, net *NetworkHarness, nodes []*HarnessNode) {
	for _, node := range nodes {
		net.SendCoins(t, btcutil.SatoshiPerBitcoin, node)
	}
}

func syncUniverses(t *testing.T, universe *tapClient, nodes ...*HarnessNode) {
	ctxb := context.Background()
	ctxt, cancel := context.WithTimeout(ctxb, defaultTimeout)
	defer cancel()

	for _, node := range nodes {
		nodeTapClient := newTapClient(t, node)

		universeHostAddr := universe.node.Cfg.LitAddr()
		t.Logf("Syncing node %v with universe %v", node.Cfg.Name,
			universeHostAddr)

		itest.SyncUniverses(
			ctxt, t, nodeTapClient, universe, universeHostAddr,
			defaultTimeout,
		)
	}
}

func assertUniverseProofExists(t *testing.T, universe *tapClient,
	assetID, scriptKey []byte, outpoint string) {

	t.Logf("Asserting proof outpoint=%v, script_key=%x", outpoint,
		scriptKey)

	req := &universerpc.UniverseKey{
		Id: &universerpc.ID{
			Id: &universerpc.ID_AssetId{
				AssetId: assetID,
			},
			ProofType: universerpc.ProofType_PROOF_TYPE_TRANSFER,
		},
		LeafKey: &universerpc.AssetKey{
			Outpoint: &universerpc.AssetKey_OpStr{
				OpStr: outpoint,
			},
			ScriptKey: &universerpc.AssetKey_ScriptKeyBytes{
				ScriptKeyBytes: scriptKey,
			},
		},
	}

	ctxb := context.Background()
	var proofResp *universerpc.AssetProofResponse
	err := wait.NoError(func() error {
		var pErr error
		proofResp, pErr = universe.QueryProof(ctxb, req)
		return pErr
	}, defaultTimeout)
	require.NoError(t, err)
	require.Equal(
		t, proofResp.AssetLeaf.Asset.AssetGenesis.AssetId, assetID,
	)
	t.Logf("Proof found for scriptKey=%x", scriptKey)
}

func assertAssetChan(t *testing.T, src, dst *HarnessNode, fundingAmount uint64,
	assetID []byte) {

	assetIDStr := hex.EncodeToString(assetID)
	err := wait.NoError(func() error {
		a, err := getChannelCustomData(src, dst)
		if err != nil {
			return err
		}

		if a.AssetInfo.AssetGenesis.AssetID != assetIDStr {
			return fmt.Errorf("expected asset ID %s, got %s",
				assetIDStr, a.AssetInfo.AssetGenesis.AssetID)
		}
		if a.Capacity != fundingAmount {
			return fmt.Errorf("expected capacity %d, got %d",
				fundingAmount, a.Capacity)
		}

		return nil
	}, defaultTimeout)
	require.NoError(t, err)
}

func assertChannelKnown(t *testing.T, node *HarnessNode,
	chanPoint *lnrpc.ChannelPoint) {

	ctxb := context.Background()
	ctxt, cancel := context.WithTimeout(ctxb, defaultTimeout)
	defer cancel()

	txid, err := chainhash.NewHash(chanPoint.GetFundingTxidBytes())
	require.NoError(t, err)
	targetChanPoint := fmt.Sprintf(
		"%v:%d", txid.String(), chanPoint.OutputIndex,
	)

	err = wait.NoError(func() error {
		graphResp, err := node.DescribeGraph(
			ctxt, &lnrpc.ChannelGraphRequest{},
		)
		if err != nil {
			return err
		}

		found := false
		for _, edge := range graphResp.Edges {
			if edge.ChanPoint == targetChanPoint {
				found = true
				break
			}
		}

		if !found {
			return fmt.Errorf("channel %v not found",
				targetChanPoint)
		}

		return nil
	}, defaultTimeout)
	require.NoError(t, err)
}

func getChannelCustomData(src, dst *HarnessNode) (*rfqmsg.JsonAssetChanInfo,
	error) {

	ctxb := context.Background()
	ctxt, cancel := context.WithTimeout(ctxb, defaultTimeout)
	defer cancel()

	srcDestChannels, err := src.ListChannels(
		ctxt, &lnrpc.ListChannelsRequest{
			Peer: dst.PubKey[:],
		},
	)
	if err != nil {
		return nil, err
	}

	if len(srcDestChannels.Channels) != 1 {
		return nil, fmt.Errorf("expected 1 channel, got %d",
			len(srcDestChannels.Channels))
	}

	targetChan := srcDestChannels.Channels[0]

	var assetData rfqmsg.JsonAssetChannel
	err = json.Unmarshal(targetChan.CustomChannelData, &assetData)
	if err != nil {
		return nil, fmt.Errorf("unable to unmarshal asset data: %w",
			err)
	}

	if len(assetData.Assets) != 1 {
		return nil, fmt.Errorf("expected 1 asset, got %d",
			len(assetData.Assets))
	}

	return &assetData.Assets[0], nil
}

func sendKeySendPayment(t *testing.T, src, dst *HarnessNode, amt uint64,
	assetID []byte) {

	ctxb := context.Background()
	ctxt, cancel := context.WithTimeout(ctxb, defaultTimeout)
	defer cancel()

	srcTapd := newTapClient(t, src)

	// Now that we know the amount we need to send, we'll convert that into
	// an HTLC tlv, which'll be used as the first hop TLV value.
	encodeReq := &tchrpc.EncodeCustomRecordsRequest_RouterSendPayment{
		RouterSendPayment: &tchrpc.RouterSendPaymentData{
			AssetAmounts: map[string]uint64{
				hex.EncodeToString(assetID): amt,
			},
		},
	}
	encodeResp, err := srcTapd.EncodeCustomRecords(
		ctxt, &tchrpc.EncodeCustomRecordsRequest{
			Input: encodeReq,
		},
	)
	require.NoError(t, err)

	// Read out the custom preimage for the keysend payment.
	var preimage lntypes.Preimage
	_, err = rand.Read(preimage[:])
	require.NoError(t, err)

	hash := preimage.Hash()

	// Set the preimage. If the user supplied a preimage with the data
	// flag, the preimage that is set here will be overwritten later.
	customRecords := make(map[uint64][]byte)
	customRecords[record.KeySendType] = preimage[:]

	const htlcCarrierAmt = 500
	req := &routerrpc.SendPaymentRequest{
		Dest:                  dst.PubKey[:],
		Amt:                   htlcCarrierAmt,
		DestCustomRecords:     customRecords,
		FirstHopCustomRecords: encodeResp.CustomRecords,
		PaymentHash:           hash[:],
		TimeoutSeconds:        3,
	}

	stream, err := src.RouterClient.SendPaymentV2(ctxt, req)
	require.NoError(t, err)

	time.Sleep(time.Second)

	result, err := getPaymentResult(stream)
	require.NoError(t, err)
	require.Equal(t, lnrpc.Payment_SUCCEEDED, result.Status)
}

func createAndPayNormalInvoice(t *testing.T, src, rfqPeer, dst *HarnessNode,
	amountSat btcutil.Amount, assetID []byte) uint64 {

	ctxb := context.Background()
	ctxt, cancel := context.WithTimeout(ctxb, defaultTimeout)
	defer cancel()

	expirySeconds := 10
	invoiceResp, err := dst.AddInvoice(ctxt, &lnrpc.Invoice{
		Value:  int64(amountSat),
		Memo:   "normal invoice",
		Expiry: int64(expirySeconds),
	})
	require.NoError(t, err)

	return payInvoiceWithAssets(t, src, rfqPeer, invoiceResp, assetID)
}

func payInvoiceWithSatoshi(t *testing.T, payer *HarnessNode,
	invoice *lnrpc.AddInvoiceResponse) {

	ctxb := context.Background()
	ctxt, cancel := context.WithTimeout(ctxb, defaultTimeout)
	defer cancel()

	sendReq := &routerrpc.SendPaymentRequest{
		PaymentRequest:   invoice.PaymentRequest,
		TimeoutSeconds:   2,
		MaxShardSizeMsat: 80_000_000,
		FeeLimitMsat:     1_000_000,
	}
	stream, err := payer.RouterClient.SendPaymentV2(ctxt, sendReq)
	require.NoError(t, err)

	time.Sleep(time.Second)

	result, err := getPaymentResult(stream)
	require.NoError(t, err)
	require.Equal(t, lnrpc.Payment_SUCCEEDED, result.Status)
}

func payInvoiceWithAssets(t *testing.T, payer, rfqPeer *HarnessNode,
	invoice *lnrpc.AddInvoiceResponse, assetID []byte) uint64 {

	ctxb := context.Background()
	ctxt, cancel := context.WithTimeout(ctxb, defaultTimeout)
	defer cancel()

	payerTapd := newTapClient(t, payer)

	decodedInvoice, err := payer.DecodePayReq(ctxt, &lnrpc.PayReqString{
		PayReq: invoice.PaymentRequest,
	})
	require.NoError(t, err)

	timeoutSeconds := uint32(60)
	resp, err := payerTapd.AddAssetSellOrder(
		ctxb, &rfqrpc.AddAssetSellOrderRequest{
			AssetSpecifier: &rfqrpc.AssetSpecifier{
				Id: &rfqrpc.AssetSpecifier_AssetId{
					AssetId: assetID,
				},
			},
			MaxAssetAmount: 25_000,
			MinAsk:         uint64(decodedInvoice.NumMsat),
			Expiry:         uint64(decodedInvoice.Expiry),
			PeerPubKey:     rfqPeer.PubKey[:],
			TimeoutSeconds: timeoutSeconds,
		},
	)
	require.NoError(t, err)

	mSatPerUnit := resp.AcceptedQuote.BidPrice
	numUnits := uint64(decodedInvoice.NumMsat) / mSatPerUnit

	t.Logf("Got quote for %v asset units at %v msat/unit from peer "+
		"%x with SCID %d", numUnits, mSatPerUnit, rfqPeer.PubKey[:],
		resp.AcceptedQuote.Scid)

	encodeReq := &tchrpc.EncodeCustomRecordsRequest_RouterSendPayment{
		RouterSendPayment: &tchrpc.RouterSendPaymentData{
			RfqId: resp.AcceptedQuote.Id,
		},
	}
	encodeResp, err := payerTapd.EncodeCustomRecords(
		ctxt, &tchrpc.EncodeCustomRecordsRequest{
			Input: encodeReq,
		},
	)
	require.NoError(t, err)

	sendReq := &routerrpc.SendPaymentRequest{
		PaymentRequest:        invoice.PaymentRequest,
		TimeoutSeconds:        2,
		FirstHopCustomRecords: encodeResp.CustomRecords,
		MaxShardSizeMsat:      80_000_000,
		FeeLimitMsat:          1_000_000,
	}
	stream, err := payer.RouterClient.SendPaymentV2(ctxt, sendReq)
	require.NoError(t, err)

	time.Sleep(time.Second)

	result, err := getPaymentResult(stream)
	require.NoError(t, err)
	require.Equal(t, lnrpc.Payment_SUCCEEDED, result.Status)

	return numUnits
}

func createAssetInvoice(t *testing.T, dstRfqPeer, dst *HarnessNode,
	assetAmount uint64, assetID []byte) *lnrpc.AddInvoiceResponse {

	ctxb := context.Background()
	ctxt, cancel := context.WithTimeout(ctxb, defaultTimeout)
	defer cancel()

	timeoutSeconds := uint32(60)
	expiry := time.Now().Add(time.Duration(timeoutSeconds) * time.Second)

	t.Logf("Asking peer %x for quote to buy assets to receive for "+
		"invoice over %d units; waiting up to %ds",
		dstRfqPeer.PubKey[:], assetAmount, timeoutSeconds)

	dstTapd := newTapClient(t, dst)
	resp, err := dstTapd.AddAssetBuyOrder(
		ctxt, &rfqrpc.AddAssetBuyOrderRequest{
			AssetSpecifier: &rfqrpc.AssetSpecifier{
				Id: &rfqrpc.AssetSpecifier_AssetId{
					AssetId: assetID,
				},
			},
			MinAssetAmount: assetAmount,
			Expiry:         uint64(expiry.Unix()),
			PeerPubKey:     dstRfqPeer.PubKey[:],
			TimeoutSeconds: timeoutSeconds,
		},
	)
	require.NoError(t, err)

	mSatPerUnit := resp.AcceptedQuote.AskPrice
	numMSats := lnwire.MilliSatoshi(assetAmount * mSatPerUnit)

	t.Logf("Got quote for %d sats at %v msat/unit from peer %x with SCID "+
		"%d", numMSats.ToSatoshis(), mSatPerUnit, dstRfqPeer.PubKey[:],
		resp.AcceptedQuote.Scid)

	peerChannels, err := dst.ListChannels(ctxt, &lnrpc.ListChannelsRequest{
		Peer: dstRfqPeer.PubKey[:],
	})
	require.NoError(t, err)
	require.Len(t, peerChannels.Channels, 1)
	peerChannel := peerChannels.Channels[0]

	ourPolicy, err := getOurPolicy(
		dst, peerChannel.ChanId, dstRfqPeer.PubKeyStr,
	)
	require.NoError(t, err)

	hopHint := &lnrpc.HopHint{
		NodeId:                    dstRfqPeer.PubKeyStr,
		ChanId:                    resp.AcceptedQuote.Scid,
		FeeBaseMsat:               uint32(ourPolicy.FeeBaseMsat),
		FeeProportionalMillionths: uint32(ourPolicy.FeeRateMilliMsat),
		CltvExpiryDelta:           ourPolicy.TimeLockDelta,
	}

	invoice := &lnrpc.Invoice{
		Memo: fmt.Sprintf("this is an asset invoice over "+
			"%d units", assetAmount),
		ValueMsat: int64(numMSats),
		Expiry:    int64(timeoutSeconds),
		RouteHints: []*lnrpc.RouteHint{
			{
				HopHints: []*lnrpc.HopHint{hopHint},
			},
		},
	}

	invoiceResp, err := dst.AddInvoice(ctxb, invoice)
	require.NoError(t, err)

	return invoiceResp
}

func closeAssetChannelAndAssert(t *harnessTest, net *NetworkHarness,
	local *HarnessNode, chanPoint *lnrpc.ChannelPoint, assetID []byte,
	universeTap *tapClient, remoteBtcBalance, remoteAssetBalance bool) {

	closeStream, closeTxid, err := t.lndHarness.CloseChannel(
		local, chanPoint, false,
	)
	require.NoError(t.t, err)

	_ = mineBlocks(t, net, 1, 1)[0]

	closeUpdate, err := t.lndHarness.WaitForChannelClose(closeStream)
	require.NoError(t.t, err)

	closeTxid, err = chainhash.NewHash(closeUpdate.ClosingTxid)
	require.NoError(t.t, err)

	closeTransaction := t.lndHarness.Miner.GetRawTransaction(closeTxid)
	closeTx := closeTransaction.MsgTx()
	t.Logf("Channel closed with txid: %v", closeTxid)
	t.Logf("Close transaction: %v", spew.Sdump(closeTx))

	// With the channel closed, we'll now assert that the co-op close
	// transaction was inserted into the local universe.
	//
	// We expect that at most four outputs exist: one for the local asset
	// output, one for the remote asset output, one for the remote BTC
	// channel balance and one for the remote BTC channel balance.
	//
	// Those outputs are only present if the respective party has a
	// non-dust balance.
	numOutputs := 2
	additionalOutputs := 1
	if remoteBtcBalance {
		numOutputs++
	}
	if remoteAssetBalance {
		numOutputs++
		additionalOutputs++
	}

	require.Len(t.t, closeTx.TxOut, numOutputs)

	outIdx := 0
	require.EqualValues(t.t, 1000, closeTx.TxOut[outIdx].Value)

	if remoteAssetBalance {
		outIdx++
		require.EqualValues(t.t, 1000, closeTx.TxOut[outIdx].Value)
	}

	// We also require there to be at most two additional outputs, one for
	// each of the asset outputs with balance.
	require.Len(t.t, closeUpdate.AdditionalOutputs, additionalOutputs)

	var remoteCloseOut *lnrpc.CloseOutput
	if remoteBtcBalance {
		// The remote node has received a couple of HTLCs with an above
		// dust value, so it should also have accumulated a non-dust
		// balance, even after subtracting 1k sats for the asset output.
		remoteCloseOut = closeUpdate.RemoteCloseOutput
		require.NotNil(t.t, remoteCloseOut)

		outIdx++
		require.EqualValues(
			t.t, remoteCloseOut.AmountSat-1000,
			closeTx.TxOut[outIdx].Value,
		)
	}

	// The local node should have received the local BTC balance minus the
	// TX fees and 1k sats for the asset output.
	localCloseOut := closeUpdate.LocalCloseOutput
	require.NotNil(t.t, localCloseOut)
	outIdx++
	require.Greater(
		t.t, closeTx.TxOut[outIdx].Value, localCloseOut.AmountSat-1000,
	)

	// Find out which of the additional outputs is the local one and which
	// is the remote.
	localAuxOut := closeUpdate.AdditionalOutputs[0]

	var remoteAuxOut *lnrpc.CloseOutput
	if remoteAssetBalance {
		remoteAuxOut = closeUpdate.AdditionalOutputs[1]
	}
	if !localAuxOut.IsLocal && remoteAuxOut != nil {
		localAuxOut, remoteAuxOut = remoteAuxOut, localAuxOut
	}

	// The first two transaction outputs should be the additional outputs
	// as identified by the pk scripts in the close update.
	localAssetIndex, remoteAssetIndex := 1, 0
	if bytes.Equal(closeTx.TxOut[0].PkScript, localAuxOut.PkScript) {
		localAssetIndex, remoteAssetIndex = 0, 1
	}

	if remoteAuxOut != nil {
		require.Equal(
			t.t, remoteAuxOut.PkScript,
			closeTx.TxOut[remoteAssetIndex].PkScript,
		)
	}

	require.Equal(
		t.t, localAuxOut.PkScript,
		closeTx.TxOut[localAssetIndex].PkScript,
	)

	// We now verify the arrival of the local balance asset proof at the
	// universe server.
	var localAssetCloseOut rfqmsg.JsonCloseOutput
	err = json.Unmarshal(
		localCloseOut.CustomChannelData, &localAssetCloseOut,
	)
	require.NoError(t.t, err)

	for assetIDStr, scriptKeyStr := range localAssetCloseOut.ScriptKeys {
		scriptKeyBytes, err := hex.DecodeString(scriptKeyStr)
		require.NoError(t.t, err)

		require.Equal(t.t, hex.EncodeToString(assetID), assetIDStr)

		assertUniverseProofExists(
			t.t, universeTap, assetID, scriptKeyBytes, fmt.Sprintf(
				"%v:%v", closeTxid, localAssetIndex,
			),
		)
	}

	// If there is no remote asset balance, we're done.
	if !remoteAssetBalance {
		return
	}

	// And then we verify the arrival of the remote balance asset proof at
	// the universe server as well.
	var remoteAssetCloseOut rfqmsg.JsonCloseOutput
	err = json.Unmarshal(
		remoteCloseOut.CustomChannelData, &remoteAssetCloseOut,
	)
	require.NoError(t.t, err)

	for assetIDStr, scriptKeyStr := range remoteAssetCloseOut.ScriptKeys {
		scriptKeyBytes, err := hex.DecodeString(scriptKeyStr)
		require.NoError(t.t, err)

		require.Equal(t.t, hex.EncodeToString(assetID), assetIDStr)

		assertUniverseProofExists(
			t.t, universeTap, assetID, scriptKeyBytes, fmt.Sprintf(
				"%v:%v", closeTxid, remoteAssetIndex,
			),
		)
	}
}

type tapClient struct {
	node *HarnessNode
	lnd  *rpc.HarnessRPC
	taprpc.TaprootAssetsClient
	assetwalletrpc.AssetWalletClient
	tapdevrpc.TapDevClient
	mintrpc.MintClient
	rfqrpc.RfqClient
	tchrpc.TaprootAssetChannelsClient
	universerpc.UniverseClient
}

func newTapClient(t *testing.T, node *HarnessNode) *tapClient {
	cfg := node.Cfg
	superMacFile, err := bakeSuperMacaroon(cfg, false)
	require.NoError(t, err)

	t.Cleanup(func() {
		require.NoError(t, os.Remove(superMacFile))
	})

	ctxb := context.Background()
	ctxt, cancel := context.WithTimeout(ctxb, defaultTimeout)
	defer cancel()

	rawConn, err := connectRPCWithMac(
		ctxt, cfg.LitAddr(), cfg.LitTLSCertPath, superMacFile,
	)
	require.NoError(t, err)

	t.Cleanup(func() {
		_ = rawConn.Close()
	})

	assetsClient := taprpc.NewTaprootAssetsClient(rawConn)
	assetWalletClient := assetwalletrpc.NewAssetWalletClient(rawConn)
	devClient := tapdevrpc.NewTapDevClient(rawConn)
	mintMintClient := mintrpc.NewMintClient(rawConn)
	rfqClient := rfqrpc.NewRfqClient(rawConn)
	tchClient := tchrpc.NewTaprootAssetChannelsClient(rawConn)
	universeClient := universerpc.NewUniverseClient(rawConn)

	return &tapClient{
		node:                       node,
		TaprootAssetsClient:        assetsClient,
		AssetWalletClient:          assetWalletClient,
		TapDevClient:               devClient,
		MintClient:                 mintMintClient,
		RfqClient:                  rfqClient,
		TaprootAssetChannelsClient: tchClient,
		UniverseClient:             universeClient,
	}
}

func connectRPCWithMac(ctx context.Context, hostPort, tlsCertPath,
	macFilePath string) (*grpc.ClientConn, error) {

	tlsCreds, err := credentials.NewClientTLSFromFile(tlsCertPath, "")
	if err != nil {
		return nil, err
	}

	opts := []grpc.DialOption{
		grpc.WithBlock(),
		grpc.WithTransportCredentials(tlsCreds),
	}

	macOption, err := readMacaroon(macFilePath)
	if err != nil {
		return nil, err
	}

	opts = append(opts, macOption)

	return grpc.DialContext(ctx, hostPort, opts...)
}

func getOurPolicy(node *HarnessNode, chanID uint64,
	remotePubKey string) (*lnrpc.RoutingPolicy, error) {

	ctxb := context.Background()
	edge, err := node.GetChanInfo(ctxb, &lnrpc.ChanInfoRequest{
		ChanId: chanID,
	})
	if err != nil {
		return nil, fmt.Errorf("unable to fetch channel: %w", err)
	}

	policy := edge.Node1Policy
	if edge.Node1Pub == remotePubKey {
		policy = edge.Node2Policy
	}

	return policy, nil
}

func assertAssetBalance(t *testing.T, client *tapClient, assetID []byte,
	expectedBalance uint64) {

	t.Helper()

	ctxb := context.Background()
	ctxt, cancel := context.WithTimeout(ctxb, shortTimeout)
	defer cancel()

	req := &taprpc.ListBalancesRequest{
		GroupBy: &taprpc.ListBalancesRequest_AssetId{
			AssetId: true,
		},
	}

	err := wait.NoError(func() error {
		assetIDBalances, err := client.ListBalances(ctxt, req)
		if err != nil {
			return err
		}

		for _, balance := range assetIDBalances.AssetBalances {
			if !bytes.Equal(balance.AssetGenesis.AssetId, assetID) {
				continue
			}

			if expectedBalance != balance.Balance {
				return fmt.Errorf("expected balance %d, got %d",
					expectedBalance, balance.Balance)
			}
		}

		return nil
	}, shortTimeout)
	require.NoError(t, err)
}

func assertAssetExists(t *testing.T, client *tapClient, assetID []byte,
	amount uint64, scriptKey *btcec.PublicKey, scriptKeyLocal,
	scriptKeyKnown, scriptKeyHasScript bool) *taprpc.Asset {

	t.Helper()

	ctxb := context.Background()
	ctxt, cancel := context.WithTimeout(ctxb, shortTimeout)
	defer cancel()

	resp, err := client.ListAssets(ctxt, &taprpc.ListAssetRequest{
		IncludeLeased: true,
	})
	require.NoError(t, err)

	for _, a := range resp.Assets {
		if !bytes.Equal(a.AssetGenesis.AssetId, assetID) {
			continue
		}

		if amount != a.Amount {
			continue
		}

		if scriptKey != nil {
			xOnlyKey, _ := schnorr.ParsePubKey(
				schnorr.SerializePubKey(scriptKey),
			)
			xOnlyKeyBytes := xOnlyKey.SerializeCompressed()
			if !bytes.Equal(xOnlyKeyBytes, a.ScriptKey) {
				continue
			}
		}

		if scriptKeyLocal != a.ScriptKeyIsLocal {
			continue
		}

		if scriptKeyKnown != a.ScriptKeyDeclaredKnown {
			continue
		}

		if scriptKeyHasScript != a.ScriptKeyHasScriptPath {
			continue
		}

		// Success, we have found the asset we're looking for.
		return a
	}

	t.Fatalf("asset with given criteria (amount=%d) not found in list, "+
		"got: %v", amount, toProtoJSON(t, resp))

	return nil
}

// readMacaroon tries to read the macaroon file at the specified path and create
// gRPC dial options from it.
func readMacaroon(macPath string) (grpc.DialOption, error) {
	// Load the specified macaroon file.
	macBytes, err := os.ReadFile(macPath)
	if err != nil {
		return nil, fmt.Errorf("unable to read macaroon path : %w", err)
	}

	return macFromBytes(macBytes)
}

// macFromBytes returns a macaroon from the given byte slice.
func macFromBytes(macBytes []byte) (grpc.DialOption, error) {
	mac := &macaroon.Macaroon{}
	if err := mac.UnmarshalBinary(macBytes); err != nil {
		return nil, fmt.Errorf("unable to decode macaroon: %w", err)
	}

	// Now we append the macaroon credentials to the dial options.
	cred, err := macaroons.NewMacaroonCredential(mac)
	if err != nil {
		return nil, fmt.Errorf("error creating macaroon credential: %w",
			err)
	}
	return grpc.WithPerRPCCredentials(cred), nil
}

func toJSON(t *testing.T, v interface{}) string {
	t.Helper()

	b, err := json.MarshalIndent(v, "", "  ")
	require.NoError(t, err)

	return string(b)
}

func toProtoJSON(t *testing.T, resp proto.Message) string {
	jsonBytes, err := taprpc.ProtoJSONMarshalOpts.Marshal(resp)
	require.NoError(t, err)

	return string(jsonBytes)
}
