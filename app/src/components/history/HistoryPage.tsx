import React from 'react';
import { observer } from 'mobx-react-lite';
import styled from '@emotion/styled';
import { usePrefixedTranslation } from 'hooks';
import { useStore } from 'store';
import PageHeader from 'components/common/PageHeader';
import HistoryList from './HistoryList';

const Styled = {
  Wrapper: styled.div`
    padding: 40px 0;
  `,
};

const HistoryPage: React.FC = () => {
  const { l } = usePrefixedTranslation('cmps.history.HistoryPage');
  const { swapStore } = useStore();

  const { Wrapper } = Styled;
  return (
    <Wrapper>
      <PageHeader title={l('pageTitle')} onExportClick={swapStore.exportSwaps} />
      <HistoryList />
    </Wrapper>
  );
};

export default observer(HistoryPage);
