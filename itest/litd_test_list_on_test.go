//go:build itest
// +build itest

package itest

var allTestCases = []*testCase{
	{
		name: "test mode integrated",
		test: testModeIntegrated,
	},
	{
		name: "test mode remote",
		test: testModeRemote,
	},
	{
		name: "stateless init mode",
		test: testStatelessInitMode,
	},
	{
		name: "test firewall rules",
		test: testFirewallRules,
	},
	{
		name: "test large http header",
		test: testLargeHttpHeader,
	},
	{
		name: "test custom channels",
		test: testCustomChannels,
	},
	{
		name: "test custom channels large",
		test: testCustomChannelsLarge,
	},
	{
		name: "test custom channels grouped asset",
		test: testCustomChannelsGroupedAsset,
	},
	{
		name: "test custom channels force close",
		test: testCustomChannelsForceClose,
	},
	{
		name: "test custom channels breach",
		test: testCustomChannelsBreach,
	},
	{
		name: "test custom channels liquidity",
		test: testCustomChannelsLiquidityEdgeCases,
	},
	{
		name: "test custom channels htlc force close",
		test: testCustomChannelsHtlcForceClose,
	},
	{
		name: "test custom channels htlc force close MPP",
		test: testCustomChannelsHtlcForceCloseMpp,
	},
	{
		name: "test custom channels balance consistency",
		test: testCustomChannelsBalanceConsistency,
	},
	{
		name: "test custom channels single asset multi input",
		test: testCustomChannelsSingleAssetMultiInput,
	},
	{
		name: "test custom channels oracle pricing",
		test: testCustomChannelsOraclePricing,
	},
	{
		name: "test custom channels fee",
		test: testCustomChannelsFee,
	},
	{
		name: "test custom channels forward bandwidth",
		test: testCustomChannelsForwardBandwidth,
	},
	{
		name: "test custom channels strict forwarding",
		test: testCustomChannelsStrictForwarding,
	},
	{
		name: "test custom channels decode payreq",
		test: testCustomChannelsDecodeAssetInvoice,
	},
}
