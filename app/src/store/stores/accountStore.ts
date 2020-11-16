import {
  keys,
  makeAutoObservable,
  observable,
  ObservableMap,
  runInAction,
  toJS,
  values,
} from 'mobx';
import { AccountState } from 'types/generated/trader_pb';
import copyToClipboard from 'copy-to-clipboard';
import { hex } from 'util/strings';
import { prefixTranslation } from 'util/translate';
import { Store } from 'store';
import { Account } from 'store/models';

const { l } = prefixTranslation('stores.AccountStore');

export default class AccountStore {
  private _store: Store;

  /** the collection of accounts */
  accounts: ObservableMap<string, Account> = observable.map();
  /** the currently active account */
  activeTraderKey?: string;

  constructor(store: Store) {
    makeAutoObservable(this, {}, { deep: false, autoBind: true });

    this._store = store;
  }

  /** returns the active account model */
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

  /** an array of accounts with opened first */
  get sortedAccounts() {
    const accts = values(this.accounts).slice();
    // sort opened accounts by the account balance
    const open = accts
      .filter(a => a.state === AccountState.OPEN)
      .sort((a, b) => {
        return +b.totalBalance.minus(a.totalBalance);
      });
    // sort unopened accounts (excluding closed) by the expiration height descending
    const other = accts
      .filter(a => a.state !== AccountState.OPEN && a.state !== AccountState.CLOSED)
      .sort((a, b) => b.expirationHeight - a.expirationHeight);
    // return the opened accounts before the unopened accounts
    return [...open, ...other];
  }

  /** the estimated amount of time until the active account expires */
  get accountExpiresIn() {
    if (!this.activeTraderKey) return 0;

    const blocksPerDay = 144;
    const currentHeight = this._store.nodeStore.blockHeight;
    const expiresHeight = this.activeAccount.expirationHeight;
    const blocks = expiresHeight - currentHeight;

    const days = Math.round(blocks / blocksPerDay);
    const weeks = Math.floor(days / 7);
    if (days <= 1) {
      return `${blocks} ${l('common.blocks', { count: blocks })}`;
    } else if (days < 14) {
      return `~${days} ${l('common.days', { count: days })}`;
    } else if (weeks < 8) {
      return `~${weeks} ${l('common.weeks', { count: weeks })}`;
    }
    const months = weeks / 4.3;

    return `~${months} ${l('common.months', { count: months.toFixed(1) })}`;
  }

  /** switch to a different account */
  setActiveTraderKey(traderKey: string) {
    this.activeTraderKey = traderKey;
    this._store.log.info(
      'updated accountStore.activeTraderKey',
      toJS(this.activeTraderKey),
    );
  }

  copyTxnId() {
    copyToClipboard(this.activeAccount.fundingTxnId);
    const msg = `Copied funding txn ID to clipboard`;
    this._store.uiStore.notify(msg, '', 'success');
  }

  /**
   * Creates an account via the pool API
   * @param amount the amount (sats) to fund the account with
   * @param expiryBlocks the number of blocks from now to expire the account
   */
  async createAccount(amount: number, expiryBlocks: number, confTarget?: number) {
    this._store.log.info(`creating new account with ${amount}sats`);
    try {
      const acct = await this._store.api.pool.initAccount(
        amount,
        expiryBlocks,
        confTarget,
      );
      runInAction(() => {
        const traderKey = hex(acct.traderKey);
        this.accounts.set(traderKey, new Account(acct));
        this.setActiveTraderKey(traderKey);
        this._store.log.info('updated accountStore.accounts', toJS(this.accounts));
      });
      return acct.traderKey;
    } catch (error) {
      this._store.uiStore.handleError(error, 'Unable to create the account');
    }
  }

  /**
   * Closes an account via the pool API
   */
  async closeAccount(feeRate: number, destination?: string) {
    try {
      const acct = this.activeAccount;
      this._store.log.info(`closing account ${acct.traderKey}`);

      const res = await this._store.api.pool.closeAccount(
        acct.traderKey,
        feeRate,
        destination,
      );
      await this.fetchAccounts();
      return res.closeTxid;
    } catch (error) {
      this._store.uiStore.handleError(error, 'Unable to close the account');
    }
  }

  /**
   * queries the pool api to fetch the list of accounts and stores them
   * in the state
   */
  async fetchAccounts() {
    this._store.log.info('fetching accounts');

    try {
      const { accountsList } = await this._store.api.pool.listAccounts();
      runInAction(() => {
        accountsList.forEach(poolAcct => {
          // update existing accounts or create new ones in state. using this
          // approach instead of overwriting the array will cause fewer state
          // mutations, resulting in better react rendering performance
          const traderKey = hex(poolAcct.traderKey);
          const existing = this.accounts.get(traderKey);
          if (existing) {
            existing.update(poolAcct);
          } else {
            this.accounts.set(traderKey, new Account(poolAcct));
          }
        });
        // remove any accounts in state that are not in the API response
        const serverIds = accountsList.map(a => hex(a.traderKey));
        const localIds = keys(this.accounts);
        localIds
          .filter(id => !serverIds.includes(id))
          .forEach(id => this.accounts.delete(id));

        // pre-select the open account with the highest balance
        if (this.sortedAccounts.length) {
          this.setActiveTraderKey(this.sortedAccounts[0].traderKey);
        }

        this._store.log.info('updated accountStore.accounts', toJS(this.accounts));
      });
    } catch (error) {
      this._store.uiStore.handleError(error, 'Unable to fetch Accounts');
    }
  }

  /**
   * submits a deposit of the specified amount to the pool api
   */
  async deposit(amount: number, feeRate?: number) {
    try {
      const acct = this.activeAccount;
      this._store.log.info(`depositing ${amount}sats into account ${acct.traderKey}`);

      const res = await this._store.api.pool.deposit(acct.traderKey, amount, feeRate);
      runInAction(() => {
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

  /**
   * submits a withdraw of the specified amount to the pool api
   */
  async withdraw(amount: number, feeRate?: number) {
    try {
      const acct = this.activeAccount;
      this._store.log.info(`withdrawing ${amount}sats into account ${acct.traderKey}`);

      const res = await this._store.api.pool.withdraw(acct.traderKey, amount, feeRate);
      runInAction(() => {
        if (res.account) {
          acct.update(res.account);
        } else {
          this.fetchAccounts();
        }
        this._store.log.info('withdraw successful', toJS(acct));
      });
      return res.withdrawTxid;
    } catch (error) {
      this._store.uiStore.handleError(error, 'Unable to withdraw funds');
    }
  }
}
