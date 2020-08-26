import * as LLM from 'types/generated/trader_pb';
import { Trader } from 'types/generated/trader_pb_service';
import BaseApi from './base';
import GrpcClient from './grpc';

/** the names and argument types for the subscription events */
// eslint-disable-next-line @typescript-eslint/no-empty-interface
interface LlmEvents {}

/**
 * An API wrapper to communicate with the LLM node via GRPC
 */
class LlmApi extends BaseApi<LlmEvents> {
  private _grpc: GrpcClient;

  constructor(grpc: GrpcClient) {
    super();
    this._grpc = grpc;
  }

  /**
   * call the LLM `InitAccount` RPC and return the response
   */
  async initAccount(
    amount: number,
    expiryBlocks: number,
    confTarget = 6,
  ): Promise<LLM.Account.AsObject> {
    const req = new LLM.InitAccountRequest();
    req.setAccountValue(amount);
    req.setRelativeHeight(expiryBlocks);
    req.setConfTarget(confTarget);
    const res = await this._grpc.request(Trader.InitAccount, req, this._meta);
    return res.toObject();
  }

  /**
   * call the LLM `ListAccounts` RPC and return the response
   */
  async listAccounts(): Promise<LLM.ListAccountsResponse.AsObject> {
    const req = new LLM.ListAccountsRequest();
    const res = await this._grpc.request(Trader.ListAccounts, req, this._meta);
    return res.toObject();
  }

  /**
   * call the LLM `DepositAccount` RPC and return the response
   */
  async deposit(
    traderKey: string,
    amount: number,
    feeRateSatPerKw = 253,
  ): Promise<LLM.DepositAccountResponse.AsObject> {
    const req = new LLM.DepositAccountRequest();
    req.setTraderKey(Buffer.from(traderKey, 'hex').toString('base64'));
    req.setAmountSat(amount);
    req.setFeeRateSatPerKw(feeRateSatPerKw);
    const res = await this._grpc.request(Trader.DepositAccount, req, this._meta);
    return res.toObject();
  }

  /**
   * call the LLM `WithdrawAccount` RPC and return the response
   */
  async withdraw(
    traderKey: string,
    amount: number,
    feeRateSatPerKw = 253,
  ): Promise<LLM.WithdrawAccountResponse.AsObject> {
    const req = new LLM.WithdrawAccountRequest();
    req.setTraderKey(Buffer.from(traderKey, 'hex').toString('base64'));
    req.setFeeRateSatPerKw(feeRateSatPerKw);
    const output = new LLM.Output();
    output.setValueSat(amount);
    req.setOutputsList([output]);
    const res = await this._grpc.request(Trader.WithdrawAccount, req, this._meta);
    return res.toObject();
  }
}

export default LlmApi;
