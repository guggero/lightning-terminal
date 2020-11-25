import React, { ReactNode } from 'react';
import { observer } from 'mobx-react-lite';
import { usePrefixedTranslation } from 'hooks';
import { styled } from 'components/theme';
import { ArrowLeft, Download, HeaderThree, HelpCircle } from '../base';
import Tip from './Tip';

const Styled = {
  Wrapper: styled.div`
    display: flex;
    justify-content: space-between;
  `,
  Left: styled.span`
    flex: 1;
    text-align: left;
  `,
  Center: styled.span`
    flex: 1;
    text-align: center;
  `,
  Right: styled.span`
    flex: 1;
    text-align: right;

    svg {
      margin-left: 20px;
    }
  `,
  BackLink: styled.a`
    text-transform: uppercase;
    font-size: ${props => props.theme.sizes.xs};
    cursor: pointer;
    line-height: 36px;

    &:hover {
      opacity: 80%;
    }
  `,
};

interface Props {
  title: ReactNode;
  onBackClick?: () => void;
  backText?: string;
  onHelpClick?: () => void;
  exportTip?: string;
  onExportClick?: () => void;
}

const PageHeader: React.FC<Props> = ({
  title,
  onBackClick,
  backText,
  onHelpClick,
  exportTip,
  onExportClick,
}) => {
  const { l } = usePrefixedTranslation('cmps.common.PageHeader');

  const { Wrapper, Left, Center, Right, BackLink } = Styled;
  return (
    <Wrapper>
      <Left>
        {onBackClick && (
          <BackLink onClick={onBackClick}>
            <ArrowLeft />
            {backText}
          </BackLink>
        )}
      </Left>
      <Center>
        <HeaderThree data-tour="welcome">{title}</HeaderThree>
      </Center>
      <Right>
        {onHelpClick && (
          <Tip placement="bottomRight" overlay={l('helpTip')}>
            <HelpCircle size="large" onClick={onHelpClick} />
          </Tip>
        )}
        {onExportClick && (
          <Tip placement="bottomRight" overlay={exportTip || l('exportTip')}>
            <Download data-tour="export" size="large" onClick={onExportClick} />
          </Tip>
        )}
      </Right>
    </Wrapper>
  );
};

export default observer(PageHeader);
