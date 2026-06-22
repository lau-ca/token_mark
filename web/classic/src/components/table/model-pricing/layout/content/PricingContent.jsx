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

import React from 'react';
import SearchActions from '../header/SearchActions';
import PricingSidebar from '../PricingSidebar';
import PricingView from './PricingView';

const PricingContent = ({ isMobile, sidebarProps, ...props }) => {
  const {
    selectedRowKeys,
    copyText,
    handleChange,
    handleCompositionStart,
    handleCompositionEnd,
    searchValue,
    t,
  } = props;

  return (
    <div className='pricing-market-shell'>
      <main className='pricing-market-main'>
        <section className='pricing-market-hero'>
          <p className='pricing-market-eyebrow'>MODEL HUB</p>
          <h1>
            {t('一站接入')}
            <span> {t('主流大模型')}</span>
          </h1>
          <p>{t('按供应商、分组、计费类型和标签筛选，统一 API 接口，即刻调用。')}</p>
        </section>

        <section className='pricing-market-toolbar'>
          <SearchActions
            selectedRowKeys={selectedRowKeys}
            copyText={copyText}
            handleChange={handleChange}
            handleCompositionStart={handleCompositionStart}
            handleCompositionEnd={handleCompositionEnd}
            isMobile={isMobile}
            searchValue={searchValue}
            showWithRecharge={sidebarProps.showWithRecharge}
            setShowWithRecharge={sidebarProps.setShowWithRecharge}
            currency={sidebarProps.currency}
            setCurrency={sidebarProps.setCurrency}
            siteDisplayType={sidebarProps.siteDisplayType}
            tokenUnit={sidebarProps.tokenUnit}
            setTokenUnit={sidebarProps.setTokenUnit}
            t={t}
          />
        </section>

        <section className='pricing-market-filter-section'>
          <PricingSidebar {...sidebarProps} />
        </section>

        <section className='pricing-market-count'>
          {t('共 {{count}} 个模型', { count: props.filteredModels?.length || 0 })}
        </section>

        <section className='pricing-market-results'>
          <PricingView {...props} viewMode='card' />
        </section>
      </main>
    </div>
  );
};

export default PricingContent;
