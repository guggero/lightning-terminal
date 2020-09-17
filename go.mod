module github.com/lightninglabs/lightning-terminal

require (
	github.com/btcsuite/btclog v0.0.0-20170628155309-84c8d2346e9f
	github.com/btcsuite/btcutil v1.0.2
	github.com/desertbit/timer v0.0.0-20180107155436-c41aec40b27f // indirect
	github.com/grpc-ecosystem/grpc-gateway v1.14.6
	github.com/improbable-eng/grpc-web v0.12.0
	github.com/jessevdk/go-flags v1.4.0
	github.com/lightninglabs/faraday v0.2.1-alpha
	github.com/lightninglabs/lndclient v0.11.0-1
	github.com/lightninglabs/loop v0.9.0-beta.0.20200917091120-4e1eaa365a44
	github.com/lightninglabs/pool v0.1.1-alpha.0.20200910042752-e289c723800e

	// TODO(guggero): Bump lnd to the final v0.11.1-beta version once it's
	// released.
	github.com/lightningnetwork/lnd v0.11.0-beta.rc4.0.20200911014924-bc6e52888763
	github.com/lightningnetwork/lnd/cert v1.0.3
	github.com/mwitkow/go-conntrack v0.0.0-20190716064945-2f068394615f
	github.com/mwitkow/grpc-proxy v0.0.0-20181017164139-0f1106ef9c76
	github.com/prometheus/client_golang v1.5.1 // indirect
	github.com/rakyll/statik v0.1.7
	github.com/rs/cors v1.7.0 // indirect
	golang.org/x/crypto v0.0.0-20200709230013-948cd5f35899
	golang.org/x/net v0.0.0-20200324143707-d3edc9973b7e // indirect
	golang.org/x/sys v0.0.0-20200406155108-e3b113bbe6a4 // indirect
	google.golang.org/grpc v1.29.1
	gopkg.in/macaroon-bakery.v2 v2.1.0
	gopkg.in/macaroon.v2 v2.1.0
)

replace github.com/lightninglabs/pool => github.com/guggero/pool v0.0.0-20200917063722-dc948a84c437

go 1.13
