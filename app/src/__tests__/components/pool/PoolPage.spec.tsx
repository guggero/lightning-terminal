import React from 'react';
import { fireEvent } from '@testing-library/react';
import { saveAs } from 'file-saver';
import { renderWithProviders } from 'util/tests';
import { createStore, Store } from 'store';
import PoolPage from 'components/pool/PoolPage';

describe('PoolPage', () => {
  let store: Store;

  beforeEach(async () => {
    store = createStore();
  });

  const render = () => {
    return renderWithProviders(<PoolPage />, store);
  };

  it('should display the title', () => {
    const { getByText } = render();
    expect(getByText('Lightning Pool')).toBeInTheDocument();
  });

  it('should export leases', async () => {
    await store.orderStore.fetchLeases();
    const { getByText } = render();
    fireEvent.click(getByText('download.svg'));
    expect(saveAs).toBeCalledWith(expect.any(Blob), 'leases.csv');
  });
});
