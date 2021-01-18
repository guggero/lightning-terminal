/* eslint-disable @typescript-eslint/no-non-null-assertion */
import React from 'react';
import * as POOL from 'types/generated/trader_pb';
import { fireEvent, waitFor } from '@testing-library/react';
import { injectIntoGrpcUnary, renderWithProviders } from 'util/tests';
import { createStore, Store } from 'store';
import { DEFAULT_MAX_BATCH_FEE, DEFAULT_MIN_CHAN_SIZE } from 'store/views/orderFormView';
import OrderFormSection from 'components/pool/OrderFormSection';

describe('OrderFormSection', () => {
  let store: Store;

  beforeEach(async () => {
    store = createStore();
    await store.accountStore.fetchAccounts();
    await store.orderStore.fetchOrders();
  });

  const render = () => {
    return renderWithProviders(<OrderFormSection />, store);
  };

  it('should display the Bid form fields', () => {
    const { getByText } = render();

    fireEvent.click(getByText('Bid'));

    expect(getByText('Desired Inbound Liquidity')).toBeInTheDocument();
    expect(getByText('Bid Premium')).toBeInTheDocument();
    expect(getByText('Minimum Channel Size')).toBeInTheDocument();
    expect(getByText('Max Batch Fee Rate')).toBeInTheDocument();
    expect(getByText('Min Node Tier')).toBeInTheDocument();
    expect(getByText('Place Bid Order')).toBeInTheDocument();
  });

  it('should display the Ask form fields', () => {
    const { getByText } = render();

    fireEvent.click(getByText('Ask'));

    expect(getByText('Offered Outbound Liquidity')).toBeInTheDocument();
    expect(getByText('Ask Premium')).toBeInTheDocument();
    expect(getByText('Minimum Channel Size')).toBeInTheDocument();
    expect(getByText('Max Batch Fee Rate')).toBeInTheDocument();
    expect(getByText('Place Ask Order')).toBeInTheDocument();
  });

  it('should toggle the additional options', () => {
    const { getByText, store } = render();

    expect(store.orderFormView.addlOptionsVisible).toBe(false);
    fireEvent.click(getByText('View Additional Options'));
    expect(store.orderFormView.addlOptionsVisible).toBe(true);
    fireEvent.click(getByText('Hide Additional Options'));
    expect(store.orderFormView.addlOptionsVisible).toBe(false);
  });

  it('should submit a bid order', async () => {
    const { getByText, changeInput, changeSelect } = render();

    changeInput('Desired Inbound Liquidity', '1000000');
    changeInput('Bid Premium', '10000');
    changeInput('Minimum Channel Size', '100000');
    changeInput('Max Batch Fee Rate', '1');
    await changeSelect('Min Node Tier', 'T0 - All Nodes');

    let bid: Required<POOL.Bid.AsObject>;
    // capture the rate that is sent to the API
    injectIntoGrpcUnary((_, props) => {
      bid = (props.request.toObject() as any).bid;
    });

    fireEvent.click(getByText('Place Bid Order'));
    expect(bid!.details.amt).toBe(1000000);
    expect(bid!.details.rateFixed).toBe(4960);
    expect(bid!.details.minUnitsMatch).toBe(1);
    expect(bid!.leaseDurationBlocks).toBe(2016);
    expect(bid!.minNodeTier).toBe(1);
    expect(bid!.details.maxBatchFeeRateSatPerKw).toBe(253);
  });

  it('should submit an ask order', async () => {
    const { getByText, changeInput } = render();

    fireEvent.click(getByText('Ask'));
    changeInput('Offered Outbound Liquidity', '1000000');
    changeInput('Ask Premium', '10000');
    changeInput('Minimum Channel Size', '100000');
    changeInput('Max Batch Fee Rate', '1');

    let ask: Required<POOL.Ask.AsObject>;
    // capture the rate that is sent to the API
    injectIntoGrpcUnary((_, props) => {
      ask = (props.request.toObject() as any).ask;
    });

    fireEvent.click(getByText('Place Ask Order'));
    expect(ask!.details.amt).toBe(1000000);
    expect(ask!.details.rateFixed).toBe(4960);
    expect(ask!.details.minUnitsMatch).toBe(1);
    expect(ask!.leaseDurationBlocks).toBe(2016);
    expect(ask!.details.maxBatchFeeRateSatPerKw).toBe(253);
  });

  it('should reset the form after placing an order', async () => {
    const { getByText, getByLabelText, changeInput } = render();
    changeInput('Desired Inbound Liquidity', '1000000');
    changeInput('Bid Premium', '10000');
    changeInput('Minimum Channel Size', '100000');
    changeInput('Max Batch Fee Rate', '1');

    fireEvent.click(getByText('Place Bid Order'));

    await waitFor(() => {
      expect(getByLabelText('Desired Inbound Liquidity')).toHaveValue('');
      expect(getByLabelText('Bid Premium')).toHaveValue('');
      expect(getByLabelText('Minimum Channel Size')).toHaveValue(
        `${DEFAULT_MIN_CHAN_SIZE}`,
      );
      expect(getByLabelText('Max Batch Fee Rate')).toHaveValue(
        `${DEFAULT_MAX_BATCH_FEE}`,
      );
    });
  });

  it('should display an error if order submission fails', async () => {
    const { getByText, findByText, changeInput } = render();

    fireEvent.click(getByText('Ask'));
    changeInput('Offered Outbound Liquidity', '1000000');
    changeInput('Ask Premium', '10000');
    changeInput('Minimum Channel Size', '100000');
    changeInput('Max Batch Fee Rate', '1');

    injectIntoGrpcUnary(() => {
      throw new Error('test-error');
    });

    fireEvent.click(getByText('Place Ask Order'));
    expect(await findByText('Unable to submit the order')).toBeInTheDocument();
    expect(await findByText('test-error')).toBeInTheDocument();
  });

  it('should display an error for amount field', () => {
    const { getByText, changeInput } = render();

    changeInput('Desired Inbound Liquidity', '1');
    expect(getByText('must be a multiple of 100,000')).toBeInTheDocument();
  });

  it('should display an error for premium field', () => {
    const { getByText, changeInput } = render();

    changeInput('Desired Inbound Liquidity', '1000000');
    changeInput('Bid Premium', '1');
    expect(getByText('per block fixed rate is too small')).toBeInTheDocument();
  });

  it('should suggest the correct premium', async () => {
    const { getByText, getByLabelText, changeInput } = render();
    await store.batchStore.fetchLatestBatch();

    store.batchStore.sortedBatches[0].clearingPriceRate = 496;
    changeInput('Desired Inbound Liquidity', '1000000');
    fireEvent.click(getByText('Suggested'));
    expect(getByLabelText('Bid Premium')).toHaveValue('1000');

    store.batchStore.sortedBatches[0].clearingPriceRate = 1884;
    changeInput('Desired Inbound Liquidity', '1000000');
    fireEvent.click(getByText('Suggested'));
    expect(getByLabelText('Bid Premium')).toHaveValue('3800');

    store.batchStore.sortedBatches[0].clearingPriceRate = 2480;
    changeInput('Desired Inbound Liquidity', '1000000');
    fireEvent.click(getByText('Suggested'));
    expect(getByLabelText('Bid Premium')).toHaveValue('5000');
  });

  it('should display an error for suggested premium', async () => {
    const { getByText, findByText, changeInput } = render();
    fireEvent.click(getByText('Suggested'));
    expect(await findByText('Unable to suggest premium')).toBeInTheDocument();
    expect(await findByText('Must specify amount first')).toBeInTheDocument();

    changeInput('Desired Inbound Liquidity', '1000000');
    fireEvent.click(getByText('Suggested'));
    expect(await findByText('Unable to suggest premium')).toBeInTheDocument();
    expect(await findByText('Previous batch not found')).toBeInTheDocument();
  });

  it('should display an error for min chan size field', () => {
    const { getByText, changeInput } = render();

    changeInput('Minimum Channel Size', '1');
    expect(getByText('must be a multiple of 100,000')).toBeInTheDocument();

    changeInput('Desired Inbound Liquidity', '1000000');
    changeInput('Minimum Channel Size', '1100000');
    expect(getByText('must be less than liquidity amount')).toBeInTheDocument();
  });

  it('should display an error for batch fee rate field', () => {
    const { getByText, changeInput } = render();

    changeInput('Max Batch Fee Rate', '0.11');
    expect(getByText('minimum 1 sats/vByte')).toBeInTheDocument();
  });

  it('should display the channel duration', () => {
    const { getByText } = render();
    expect(getByText('Channel Duration (blocks)')).toBeInTheDocument();
    expect(getByText('2016')).toBeInTheDocument();
    expect(getByText('(~2 wks)')).toBeInTheDocument();
  });

  it('should calculate the per block rate', () => {
    const { getByText, changeInput } = render();

    expect(getByText('Per Block Fixed Rate')).toBeInTheDocument();

    changeInput('Desired Inbound Liquidity', '1000000');
    changeInput('Bid Premium', '1000');
    expect(getByText('496')).toBeInTheDocument();

    changeInput('Desired Inbound Liquidity', '5000000');
    changeInput('Bid Premium', '1000');
    expect(getByText('99')).toBeInTheDocument();

    changeInput('Desired Inbound Liquidity', '50000000');
    changeInput('Bid Premium', '100');
    expect(getByText('< 1')).toBeInTheDocument();
  });

  it('should calculate the interest rate percent correctly', () => {
    const { getByText, getAllByText, changeInput } = render();

    expect(getByText('Interest Rate Percent')).toBeInTheDocument();

    changeInput('Desired Inbound Liquidity', '1000000');
    changeInput('Bid Premium', '1000');
    expect(getByText('0.1%')).toBeInTheDocument();

    changeInput('Desired Inbound Liquidity', '1000000');
    changeInput('Bid Premium', '500');
    expect(getByText('0.05%')).toBeInTheDocument();

    changeInput('Desired Inbound Liquidity', '1000000');
    changeInput('Bid Premium', '1234');
    expect(getByText('0.12%')).toBeInTheDocument();

    changeInput('Desired Inbound Liquidity', '1000000');
    changeInput('Bid Premium', '');
    expect(getAllByText('0%')).toHaveLength(2);
  });

  it('should calculate the APR correctly', () => {
    const { getByText, getAllByText, changeInput } = render();

    expect(getByText('Annual Rate (APR)')).toBeInTheDocument();

    changeInput('Desired Inbound Liquidity', '1000000');
    changeInput('Bid Premium', '1000');
    expect(getByText('2.61%')).toBeInTheDocument();

    changeInput('Desired Inbound Liquidity', '1000000');
    changeInput('Bid Premium', '500');
    expect(getByText('1.3%')).toBeInTheDocument();

    changeInput('Desired Inbound Liquidity', '1000000');
    changeInput('Bid Premium', '1234');
    expect(getByText('3.22%')).toBeInTheDocument();

    changeInput('Desired Inbound Liquidity', '1000000');
    changeInput('Bid Premium', '');
    expect(getAllByText('0%')).toHaveLength(2);
  });
});
