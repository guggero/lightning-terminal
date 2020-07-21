package terminal

import "gopkg.in/macaroon-bakery.v2/bakery"

var (
	// faradayPermissions is a map of all faraday RPC methods and their
	// required macaroon permissions.
	//
	// TODO(guggero): Move to faraday repo once macaroons are enabled there
	// and use more application specific permissions.
	faradayPermissions = map[string][]bakery.Op{
		"/frdrpc.FaradayServer/OutlierRecommendations": {{
			Entity: "offchain",
			Action: "read",
		}},
		"/frdrpc.FaradayServer/ThresholdRecommendations": {{
			Entity: "offchain",
			Action: "read",
		}},
		"/frdrpc.FaradayServer/RevenueReport": {{
			Entity: "offchain",
			Action: "read",
		}},
		"/frdrpc.FaradayServer/ChannelInsights": {{
			Entity: "offchain",
			Action: "read",
		}},
	}

	// loopPermissions is a map of all loop RPC methods and their required
	// macaroon permissions.
	//
	// TODO(guggero): Move to loop repo once macaroons are enabled there
	// and use more application specific permissions.
	loopPermissions = map[string][]bakery.Op{
		"/looprpc.SwapClient/LoopOut": {{
			Entity: "offchain",
			Action: "read",
		}},
		"/looprpc.SwapClient/LoopIn": {{
			Entity: "offchain",
			Action: "read",
		}},
		"/looprpc.SwapClient/Monitor": {{
			Entity: "offchain",
			Action: "read",
		}},
		"/looprpc.SwapClient/ListSwaps": {{
			Entity: "offchain",
			Action: "read",
		}},
		"/looprpc.SwapClient/SwapInfo": {{
			Entity: "offchain",
			Action: "read",
		}},
		"/looprpc.SwapClient/LoopOutTerms": {{
			Entity: "offchain",
			Action: "read",
		}},
		"/looprpc.SwapClient/LoopOutQuote": {{
			Entity: "offchain",
			Action: "read",
		}},
		"/looprpc.SwapClient/GetLoopInTerms": {{
			Entity: "offchain",
			Action: "read",
		}},
		"/looprpc.SwapClient/GetLoopInQuote": {{
			Entity: "offchain",
			Action: "read",
		}},
		"/looprpc.SwapClient/GetLsatTokens": {{
			Entity: "offchain",
			Action: "read",
		}},
	}

	// llmPermissions is a map of all llm RPC methods and their required
	// macaroon permissions.
	//
	// TODO(guggero): Move to llm repo once macaroons are enabled there
	// and use more application specific permissions.
	llmPermissions = map[string][]bakery.Op{
		"/clmrpc.Trader/InitAccount": {{
			Entity: "onchain",
			Action: "write",
		}},
		"/clmrpc.Trader/ListAccounts": {{
			Entity: "onchain",
			Action: "read",
		}},
		"/clmrpc.Trader/CloseAccount": {{
			Entity: "onchain",
			Action: "write",
		}},
		"/clmrpc.Trader/WithdrawAccount": {{
			Entity: "onchain",
			Action: "write",
		}},
		"/clmrpc.Trader/DepositAccount": {{
			Entity: "onchain",
			Action: "write",
		}},
		"/clmrpc.Trader/RecoverAccounts": {{
			Entity: "onchain",
			Action: "write",
		}},
		"/clmrpc.Trader/SubmitOrder": {{
			Entity: "offchain",
			Action: "read",
		}},
		"/clmrpc.Trader/ListOrders": {{
			Entity: "offchain",
			Action: "read",
		}},
		"/clmrpc.Trader/CancelOrder": {{
			Entity: "offchain",
			Action: "read",
		}},
		"/clmrpc.Trader/AuctionFee": {{
			Entity: "offchain",
			Action: "read",
		}},
		"/clmrpc.Trader/BatchSnapshot": {{
			Entity: "offchain",
			Action: "read",
		}},
	}
)

// getSubserverPermissions returns a merged map of all subserver macaroon
// permissions.
func getSubserverPermissions() map[string][]bakery.Op {
	mapSize := len(faradayPermissions) + len(loopPermissions) +
		len(llmPermissions)
	result := make(map[string][]bakery.Op, mapSize)
	for key, value := range faradayPermissions {
		result[key] = value
	}
	for key, value := range loopPermissions {
		result[key] = value
	}
	for key, value := range llmPermissions {
		result[key] = value
	}
	return result
}
