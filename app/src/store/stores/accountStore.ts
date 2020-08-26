import {
  action,
  computed,
  observable,
  ObservableMap,
  runInAction,
  toJS,
  values,
} from 'mobx';
import { hex } from 'util/strings';
import { Store } from 'store';
import { Account } from 'store/models';

export default class AccountStore {
  private _store: Store;

  /** the collection of accounts */
  @observable accounts: ObservableMap<string, Account> = observable.map();
  /** the currently active account */
  @observable activeTraderKey?: string;

  constructor(store: Store) {
    this._store = store;
  }

  /** returns the active account model */
  @computed
  get activeAccount() {
    if (!this.activeTraderKey) {
      throw new Error('Select an account to manage');
    }

    const account = this.accounts.get(this.activeTraderKey);
    if (!account) {
      throw new Error('The selected account is not valid');
    }

    return account;
  }

  /** switch to a different account */
  @action.bound
  setActiveTraderKey(traderKey: string) {
    this.activeTraderKey = traderKey;
  }

  /**
   * Creates an account via the LLM API
   * @param amount the amount (sats) to fund the account with
   * @param expiryBlocks the number of blocks from now to expire the account
   */
  @action.bound
  async createAccount(amount: number, expiryBlocks: number, confTarget?: number) {
    this._store.log.info(`creating new account with ${amount}sats`);
    try {
      const acct = await this._store.api.llm.initAccount(
        amount,
        expiryBlocks,
        confTarget,
      );
      runInAction('createAccountContinuation', () => {
        const traderKey = hex(acct.traderKey);
        this.accounts.set(traderKey, new Account(acct));
        this.setActiveTraderKey(traderKey);
        this._store.log.info('updated accountStore.accounts', toJS(this.accounts));
      });
    } catch (error) {
      this._store.uiStore.handleError(error, 'Unable to create the account');
    }
  }

  /**
   * queries the LLM api to fetch the list of accounts and stores them
   * in the state
   */
  @action.bound
  async fetchAccounts() {
    this._store.log.info('fetching accounts');

    try {
      const { accountsList } = await this._store.api.llm.listAccounts();
      runInAction('fetchAccountsContinuation', () => {
        accountsList.forEach(llmAcct => {
          // update existing accounts or create new ones in state. using this
          // approach instead of overwriting the array will cause fewer state
          // mutations, resulting in better react rendering performance
          const traderKey = hex(llmAcct.traderKey);
          const existing = this.accounts.get(traderKey);
          if (existing) {
            existing.update(llmAcct);
          } else {
            this.accounts.set(traderKey, new Account(llmAcct));
          }
        });
        // remove any accounts in state that are not in the API response
        const serverIds = accountsList.map(a => hex(a.traderKey));
        const localIds = Object.keys(this.accounts);
        localIds
          .filter(id => !serverIds.includes(id))
          .forEach(id => this.accounts.delete(id));

        // pre-select the open account with the highest balance
        const account = values(this.accounts)
          .slice()
          .sort((a, b) => +b.totalBalance.sub(a.totalBalance))
          .find(a => a.stateLabel === 'Open');
        if (account) {
          this.setActiveTraderKey(account.traderKey);
        }

        this._store.log.info('updated accountStore.accounts', toJS(this.accounts));
      });
    } catch (error) {
      this._store.uiStore.handleError(error, 'Unable to fetch Accounts');
    }
  }

  /**
   * submits a deposit of the specified amount to the LLM api
   */
  @action.bound
  async deposit(amount: number, feeRate?: number) {
    try {
      const acct = this.activeAccount;
      this._store.log.info(`depositing ${amount}sats into account ${acct.traderKey}`);

      const res = await this._store.api.llm.deposit(acct.traderKey, amount, feeRate);
      runInAction('depositContinuation', () => {
        // the account should always be defined but if not, fetch all accounts as a fallback
        if (res.account) {
          acct.update(res.account);
        } else {
          this.fetchAccounts();
        }
        this._store.log.info('deposit successful', toJS(acct));
      });
      return res.depositTxid;
    } catch (error) {
      this._store.uiStore.handleError(error, 'Unable to deposit funds');
    }
  }
}
