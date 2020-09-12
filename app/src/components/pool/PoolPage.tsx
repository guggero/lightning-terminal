import React from 'react';
import { observer } from 'mobx-react-lite';
import { usePrefixedTranslation } from 'hooks';
import { Column, Row } from 'components/base';
import PageHeader from 'components/common/PageHeader';
import { styled } from 'components/theme';
import AccountSection from './AccountSection';
import ChartSection from './ChartSection';
import DetailsSection from './DetailsSection';
import OrderFormSection from './OrderFormSection';

const Styled = {
  Wrapper: styled.div`
    padding: 40px 0 0;
    height: 100vh;
    display: flex;
    flex-direction: column;
  `,
  Row: styled(Row)`
    flex: 1;
  `,
  Col: styled(Column)`
    display: flex;
    flex-direction: column;

    &:first-of-type {
      width: 400px;
    }
  `,
};

const PoolPage: React.FC = () => {
  const { l } = usePrefixedTranslation('cmps.pool.PoolPage');

  const { Wrapper, Row, Col } = Styled;
  return (
    <Wrapper>
      <PageHeader title={l('pageTitle')} />
      <Row>
        <Col cols={4} colsXl={3}>
          <AccountSection />
          <OrderFormSection />
        </Col>
        <Col>
          <ChartSection />
          <DetailsSection />
        </Col>
      </Row>
    </Wrapper>
  );
};

export default observer(PoolPage);