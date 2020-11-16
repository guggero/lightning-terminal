import React from 'react';
import { Router } from 'react-router';
import { fireEvent, render } from '@testing-library/react';
import { createStore, Store, StoreProvider } from 'store';
import AlertContainer from 'components/common/AlertContainer';
import { ThemeProvider } from 'components/theme';

/**
 * Renders a component inside of the theme and mobx store providers
 * to supply context items needed to render some child components
 * @param component the component under test to render
 * @param withStore the store to use in the provider
 */
const renderWithProviders = (component: React.ReactElement, withStore?: Store) => {
  const store = withStore || createStore();
  const result = render(
    <StoreProvider store={store}>
      <ThemeProvider>
        <Router history={store.router.history}>{component}</Router>
        <AlertContainer />
      </ThemeProvider>
    </StoreProvider>,
  );

  const changeInput = (label: string, value: string) => {
    fireEvent.change(result.getByLabelText(label), { target: { value } });
  };

  return { ...result, store, changeInput };
};

export default renderWithProviders;
