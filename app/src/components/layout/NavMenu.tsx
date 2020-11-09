import React from 'react';
import { observer } from 'mobx-react-lite';
import { usePrefixedTranslation } from 'hooks';
import { useStore } from 'store';
import { HeaderFour } from 'components/base';
import { styled } from 'components/theme';

const Styled = {
  NavHeader: styled(HeaderFour)`
    padding: 8px 14px;
  `,
  Nav: styled.ul`
    padding-left: 0;
    margin-bottom: 0;
    list-style: none;
  `,
  NavItem: styled.li`
    font-size: ${props => props.theme.sizes.xs};
    margin-right: -17px;

    span {
      display: block;
      height: 50px;
      line-height: 50px;
      padding: 0 12px;
      border-left: 3px solid transparent;
      color: ${props => props.theme.colors.offWhite};
      cursor: pointer;

      &:hover {
        text-decoration: none;
        border-left: 3px solid ${props => props.theme.colors.pink};
        background-color: ${props => props.theme.colors.overlay};
      }
    }

    &.active span {
      border-left: 3px solid ${props => props.theme.colors.offWhite};
      background-color: ${props => props.theme.colors.blue};

      &:hover {
        border-left: 3px solid ${props => props.theme.colors.pink};
      }
    }
  `,
};

const NavItem: React.FC<{ page: string; onClick: () => void }> = observer(
  ({ page, onClick }) => {
    const { l } = usePrefixedTranslation('cmps.layout.NavMenu');
    const { router } = useStore();
    const className = router.location.pathname === `/${page}` ? 'active' : '';

    return (
      <Styled.NavItem className={className}>
        <span onClick={onClick}>{l(page)}</span>
      </Styled.NavItem>
    );
  },
);

const NavMenu: React.FC = () => {
  const { l } = usePrefixedTranslation('cmps.layout.NavMenu');
  const { appView } = useStore();

  const { NavHeader, Nav } = Styled;
  return (
    <>
      <NavHeader>{l('menu')}</NavHeader>
      <Nav>
        <NavItem page="loop" onClick={appView.goToLoop} />
        <NavItem page="history" onClick={appView.goToHistory} />
        <NavItem page="pool" onClick={appView.goToPool} />
        <NavItem page="settings" onClick={appView.goToSettings} />
      </Nav>
    </>
  );
};

export default observer(NavMenu);
