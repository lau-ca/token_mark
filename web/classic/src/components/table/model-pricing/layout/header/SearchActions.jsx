/*
Copyright (C) 2025 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/

import React, { memo, useCallback } from 'react';
import { Input, Button, Switch, Select, Divider } from '@douyinfe/semi-ui';
import { IconSearch, IconCopy } from '@douyinfe/semi-icons';

const SearchActions = memo(
  ({
    selectedRowKeys = [],
    copyText,
    handleChange,
    handleCompositionStart,
    handleCompositionEnd,
    isMobile = false,
    searchValue = '',
    showWithRecharge,
    setShowWithRecharge,
    currency,
    setCurrency,
    siteDisplayType,
    tokenUnit,
    setTokenUnit,
    t,
  }) => {
    const supportsCurrencyDisplay = siteDisplayType !== 'TOKENS';

    const handleCopyClick = useCallback(() => {
      if (copyText && selectedRowKeys.length > 0) {
        copyText(selectedRowKeys);
      }
    }, [copyText, selectedRowKeys]);

    const handleTokenUnitToggle = useCallback(() => {
      setTokenUnit?.(tokenUnit === 'K' ? 'M' : 'K');
    }, [tokenUnit, setTokenUnit]);

    return (
      <div className='flex items-center gap-2 w-full'>
        <div className='flex-1'>
          <Input
            prefix={<IconSearch />}
            placeholder={t('模糊搜索模型名称')}
            value={searchValue}
            onCompositionStart={handleCompositionStart}
            onCompositionEnd={handleCompositionEnd}
            onChange={handleChange}
            showClear
          />
        </div>

        <Button
          theme='outline'
          type='tertiary'
          icon={<IconCopy />}
          onClick={handleCopyClick}
          disabled={selectedRowKeys.length === 0}
          className='pricing-market-copy-button'
        >
          {t('复制')}
        </Button>

        {!isMobile && (
          <>
            <Divider layout='vertical' margin='8px' />

            {/* 充值价格显示开关 */}
            {supportsCurrencyDisplay && (
              <div className='flex items-center gap-2'>
                <span className='text-sm text-gray-600'>{t('充值价格显示')}</span>
                <Switch
                  checked={showWithRecharge}
                  onChange={setShowWithRecharge}
                />
              </div>
            )}

            {/* 货币单位选择 */}
            {supportsCurrencyDisplay && showWithRecharge && (
              <Select
                value={currency}
                onChange={setCurrency}
                optionList={[
                  { value: 'USD', label: 'USD' },
                  { value: 'CNY', label: 'CNY' },
                  { value: 'CUSTOM', label: t('自定义货币') },
                ]}
              />
            )}

            {/* Token单位切换按钮 */}
            <Button
              theme={tokenUnit === 'K' ? 'solid' : 'outline'}
              type={tokenUnit === 'K' ? 'primary' : 'tertiary'}
              onClick={handleTokenUnitToggle}
            >
              {tokenUnit}
            </Button>
          </>
        )}
      </div>
    );
  },
);

SearchActions.displayName = 'SearchActions';

export default SearchActions;
