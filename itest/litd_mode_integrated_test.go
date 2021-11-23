package itest

import (
	"context"
	"time"

	"github.com/btcsuite/btcutil"
	"github.com/davecgh/go-spew/spew"
	"github.com/lightningnetwork/lnd/lnrpc"
	"github.com/stretchr/testify/require"
)

// testModeIntegrated makes sure that in integrated mode all daemons work
// correctly.
func testModeIntegrated(net *NetworkHarness, t *harnessTest) {
	ctx := context.Background()

	time.Sleep(2 * time.Second)

	net.SendCoins(t.t, btcutil.SatoshiPerBitcoin, net.Alice)

	resp, err := net.Alice.GetInfo(ctx, &lnrpc.GetInfoRequest{})
	require.NoError(t.t, err)

	require.NotEmpty(t.t, spew.Sdump(resp))
}
