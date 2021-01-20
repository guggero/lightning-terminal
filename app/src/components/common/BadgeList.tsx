import React, { useCallback } from 'react';
import { Badge } from 'components/base';
import { styled } from 'components/theme';

const Styled = {
  Wrapper: styled.div<{ flex?: boolean }>`
    display: inline-block;
  `,
  Badge: styled(Badge)<{ selected?: boolean }>`
    font-family: ${props => props.theme.fonts.open.regular};
    font-size: ${props => props.theme.sizes.xs};
    color: ${props =>
      props.selected ? props.theme.colors.offWhite : props.theme.colors.gray};
    border: 1px solid
      ${props => (props.selected ? props.theme.colors.offWhite : props.theme.colors.gray)};

    padding: 5px 10px;
    letter-spacing: normal;
    cursor: pointer;

    &:hover {
      opacity: 1;
      background-color: ${props => props.theme.colors.overlay};
      color: ${props => props.theme.colors.offWhite};
      border-color: ${props => props.theme.colors.offWhite};
    }
  `,
};

export interface BadgeListOption {
  label: string;
  value: string;
}

interface Props {
  options: BadgeListOption[];
  value?: string;
  onChange: (value: string) => void;
}

const BadgeList: React.FC<Props> = ({ options, value, onChange }) => {
  const handleClick = useCallback((v: string) => () => onChange(v), [onChange]);

  const { Wrapper, Badge } = Styled;
  return (
    <Wrapper>
      {options.map(o => (
        <Badge key={o.value} selected={o.value === value} onClick={handleClick(o.value)}>
          {o.label}
        </Badge>
      ))}
    </Wrapper>
  );
};

export default BadgeList;
