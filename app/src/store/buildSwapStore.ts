import { action, computed, observable, runInAction, toJS } from 'mobx';
import { Quote, SwapDirection, SwapTerms } from 'types/state';
import { Store } from 'store';
import { Channel } from './models';

/**
 * The store to manage the state of a Loop swap being created
 */
class BuildSwapStore {
  _store: Store;

  /** determines whether to show the options for Loop In or Loop Out */
  @observable showActions = false;

  /** determines whether to show the swap wizard */
  @observable showWizard = false;

  /** determines whether to show the swap wizard */
  @observable currentStep = 1;

  /** the chosen direction for the loop */
  @observable direction: SwapDirection = SwapDirection.IN;

  /** the channels selected */
  @observable channels: Channel[] = [];

  /** the amount to swap */
  @observable amount = 0;

  /** the min/max amount this node is allowed to swap */
  @observable terms: SwapTerms = {
    in: { min: 0, max: 0 },
    out: { min: 0, max: 0 },
  };

  /** the quote for the swap */
  @observable quote: Quote = {
    swapFee: 0,
    prepayAmount: 0,
    minerFee: 0,
  };

  /** the reference to the timeout used to allow cancelling a swap */
  @observable processingTimeout?: NodeJS.Timeout;

  /** the error error returned from LoopIn/LoopOut if any */
  @observable swapError?: Error;

  constructor(store: Store) {
    this._store = store;
  }

  //
  // Computed properties
  //

  /** determines if the channel list should be editable */
  @computed
  get listEditable(): boolean {
    return this.showActions || this.showWizard;
  }

  /** the min/max amounts this node is allowed to swap */
  @computed
  get termsMinMax() {
    let termsMax = { min: 0, max: 0 };
    switch (this.direction) {
      case SwapDirection.IN:
        termsMax = this.terms.in;
        break;
      case SwapDirection.OUT:
        termsMax = this.terms.out;
        break;
    }

    return termsMax;
  }

  /** the total quoted fee to perform the current swap */
  @computed
  get fee() {
    return this.quote.swapFee + this.quote.minerFee;
  }

  //
  // Actions
  //

  /**
   * display the Loop actions bar
   */
  @action.bound
  toggleShowActions() {
    this.showActions = !this.showActions;
    this.getTerms();
  }

  /**
   * Set the direction, In or Out, for the pending swap
   * @param direction the direction of the swap
   */
  @action.bound
  setDirection(direction: SwapDirection) {
    this.direction = direction;
    this.showActions = false;
    this.showWizard = true;
    const { min, max } = this.termsMinMax;
    this.amount = Math.floor((min + max) / 2);
  }

  /**
   * Set the selected channels to use for the pending swap
   * @param channels the selected channels
   */
  @action.bound
  setSelectedChannels(channels: Channel[]) {
    this.channels = channels;
  }

  /**
   * Set the amount for the swap
   * @param amount the amount in sats
   */
  @action.bound
  setAmount(amount: number) {
    this.amount = amount;
  }

  /**
   * Navigate to the next step in the wizard
   */
  @action.bound
  goToNextStep() {
    if (this.currentStep === 1) {
      this.getQuote();
    } else if (this.currentStep === 2) {
      this.executeSwap();
    }

    this.currentStep++;
  }

  /**
   * Navigate to the previous step in the wizard
   */
  @action.bound
  goToPrevStep() {
    if (this.currentStep === 1) {
      this.showActions = true;
      this.showWizard = false;
    } else {
      if (this.currentStep === 3) {
        // if back is clicked on the processing step
        this.abortSwap();
      }
      this.currentStep--;
    }
  }

  /**
   * hide the swap wizard
   */
  @action.bound
  cancel() {
    this.showActions = false;
    this.showWizard = false;
    this.channels = [];
    this.quote.swapFee = 0;
    this.quote.minerFee = 0;
    this.quote.prepayAmount = 0;
    this.currentStep = 1;
  }

  /**
   * fetch the terms, minimum/maximum swap amount allowed, from the Loop API
   */
  @action.bound
  async getTerms() {
    this._store.log.info(`fetching loop terms`);
    const inTerms = await this._store.api.loop.getLoopInTerms();
    const outTerms = await this._store.api.loop.getLoopOutTerms();
    this.terms = {
      in: {
        min: inTerms.minSwapAmount,
        max: inTerms.maxSwapAmount,
      },
      out: {
        min: outTerms.minSwapAmount,
        max: outTerms.maxSwapAmount,
      },
    };
    this._store.log.info('updated store.terms', toJS(this.terms));
  }

  /**
   * get a loop quote from the Loop RPC
   */
  @action.bound
  async getQuote() {
    const { amount, direction } = this;
    this._store.log.info(`fetching ${direction} quote for ${amount} sats`);

    const quote =
      direction === SwapDirection.IN
        ? await this._store.api.loop.getLoopInQuote(amount)
        : await this._store.api.loop.getLoopOutQuote(amount);

    this.quote = {
      swapFee: quote.swapFee,
      minerFee: quote.minerFee,
      prepayAmount: quote.prepayAmt,
    };
    this._store.log.info('updated buildSwapStore.quote', toJS(this.quote));
  }

  /**
   * submit a request to the Loop API to perform a swap. There will be a 3 second
   * delay added to allow the swap to be aborted
   */
  @action.bound
  executeSwap() {
    const delay = process.env.NODE_ENV !== 'test' ? 3000 : 1;
    const { amount, direction, quote } = this;
    this._store.log.info(`executing ${direction} for ${amount} sats in ${delay}ms`);
    this.processingTimeout = setTimeout(() => {
      runInAction(async () => {
        try {
          const res =
            direction === SwapDirection.IN
              ? await this._store.api.loop.loopIn(amount, quote)
              : await this._store.api.loop.loopOut(amount, quote);
          this._store.log.info('completed loop', toJS(res));
          // hide the swap UI after it is complete
          this.cancel();
        } catch (error) {
          this.swapError = error;
          this._store.log.error(`failed to perform ${direction}`, error);
        }
      });
    }, delay);
  }

  /**
   * abort a swap that has been submitted
   */
  @action.bound
  abortSwap() {
    if (this.processingTimeout) {
      clearTimeout(this.processingTimeout);
      this.processingTimeout = undefined;
    }
  }
}

export default BuildSwapStore;
