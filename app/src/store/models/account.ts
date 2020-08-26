import { action, computed, observable } from 'mobx';
import * as LLM from 'types/generated/trader_pb';
import Big from 'big.js';
import { hex } from 'util/strings';

export default class Account {
  // native values from the LLM api
  @observable traderKey = '';
  @observable totalBalance = Big(0);
  @observable availableBalance = Big(0);
  @observable expirationHeight = 0;
  @observable state = 0;

  constructor(llmAccount: LLM.Account.AsObject) {
    this.update(llmAccount);
  }

  /**
   * The numeric account `state` as a user friendly string
   */
  @computed get stateLabel() {
    switch (this.state) {
      case LLM.AccountState.PENDING_OPEN:
        return 'Pending Open';
      case LLM.AccountState.PENDING_UPDATE:
        return 'Pending Update';
      case LLM.AccountState.OPEN:
        return 'Open';
      case LLM.AccountState.EXPIRED:
        return 'Expired';
      case LLM.AccountState.PENDING_CLOSED:
        return 'Pending Closed';
      case LLM.AccountState.CLOSED:
        return 'Closed';
      case LLM.AccountState.RECOVERY_FAILED:
        return 'Recovery Failed';
    }

    return 'Unknown';
  }

  /**
   * Updates this account model using data provided from the LLM GRPC api
   * @param llmAccount the account data
   */
  @action.bound
  update(llmAccount: LLM.Account.AsObject) {
    this.traderKey = hex(llmAccount.traderKey);
    this.totalBalance = Big(llmAccount.value);
    this.availableBalance = Big(llmAccount.availableBalance);
    this.expirationHeight = llmAccount.expirationHeight;
    this.state = llmAccount.state;
  }
}
