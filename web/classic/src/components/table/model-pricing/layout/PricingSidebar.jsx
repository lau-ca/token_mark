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
import { Button } from '@douyinfe/semi-ui';
import { resetPricingFilters } from '../../../../helpers/utils';
import { usePricingFilterCounts } from '../../../../hooks/model-pricing/usePricingFilterCounts';
import { getLobeHubIcon } from '../../../../helpers';

const PricingFilterRow = ({ label, items, activeValue, onChange }) => (
  <div className='pricing-filter-row'>
    <div className='pricing-filter-row-label'>{label}</div>
    <div className='pricing-filter-options'>
      {items.map((item) => {
        const active = activeValue === item.value;
        return (
          <button
            key={String(item.value)}
            type='button'
            onClick={() => onChange(item.value)}
            className={`pricing-filter-chip ${active ? 'is-active' : ''}`}
          >
            {item.icon && <span className='pricing-filter-chip-icon'>{item.icon}</span>}
            <span className='pricing-filter-chip-label'>{item.label}</span>
            {item.tagCount !== undefined && item.tagCount !== '' && (
              <span className='pricing-filter-chip-count'>{item.tagCount}</span>
            )}
          </button>
        );
      })}
    </div>
  </div>
);

const PricingSidebar = ({
  showWithRecharge,
  setShowWithRecharge,
  currency,
  setCurrency,
  handleChange,
  setActiveKey,
  viewMode,
  setViewMode,
  filterGroup,
  setFilterGroup,
  handleGroupClick,
  filterQuotaType,
  setFilterQuotaType,
  filterEndpointType,
  setFilterEndpointType,
  filterVendor,
  setFilterVendor,
  filterTag,
  setFilterTag,
  currentPage,
  setCurrentPage,
  tokenUnit,
  setTokenUnit,
  loading,
  t,
  ...categoryProps
}) => {
  const [expanded, setExpanded] = React.useState(false);
  const {
    quotaTypeModels,
    endpointTypeModels,
    vendorModels,
    tagModels,
    groupCountModels,
  } = usePricingFilterCounts({
    models: categoryProps.models,
    filterGroup,
    filterQuotaType,
    filterEndpointType,
    filterVendor,
    filterTag,
    searchValue: categoryProps.searchValue,
  });

  const handleResetFilters = () =>
    resetPricingFilters({
      handleChange,
      setShowWithRecharge,
      setCurrency,
      setViewMode,
      setFilterGroup,
      setFilterQuotaType,
      setFilterEndpointType,
      setFilterVendor,
      setFilterTag,
      setCurrentPage,
      setTokenUnit,
    });

  const vendorItems = React.useMemo(() => {
    const vendors = new Set();
    const vendorIcons = new Map();
    let hasUnknownVendor = false;

    (categoryProps.models || []).forEach((model) => {
      if (model.vendor_name) {
        vendors.add(model.vendor_name);
        if (model.vendor_icon && !vendorIcons.has(model.vendor_name)) {
          vendorIcons.set(model.vendor_name, model.vendor_icon);
        }
      } else {
        hasUnknownVendor = true;
      }
    });

    const getVendorCount = (vendor) => {
      if (vendor === 'all') return vendorModels.length;
      if (vendor === 'unknown') {
        return vendorModels.filter((model) => !model.vendor_name).length;
      }
      return vendorModels.filter((model) => model.vendor_name === vendor).length;
    };

    const items = [
      { value: 'all', label: t('全部供应商'), tagCount: getVendorCount('all') },
    ];

    Array.from(vendors)
      .sort()
      .forEach((vendor) => {
        const icon = vendorIcons.get(vendor);
        items.push({
          value: vendor,
          label: vendor,
          icon: icon ? getLobeHubIcon(icon, 16) : null,
          tagCount: getVendorCount(vendor),
        });
      });

    if (hasUnknownVendor) {
      items.push({
        value: 'unknown',
        label: t('未知供应商'),
        tagCount: getVendorCount('unknown'),
      });
    }

    return items;
  }, [categoryProps.models, t, vendorModels]);

  const groupItems = React.useMemo(
    () =>
      ['all', ...Object.keys(categoryProps.usableGroup || {}).filter((key) => key !== '')].map((group) => {
        const count =
          group === 'all'
            ? groupCountModels.length
            : groupCountModels.filter((model) => model.enable_groups?.includes(group)).length;
        const ratio = group === 'all' ? count : `${categoryProps.groupRatio?.[group] ?? 1}x`;
        return {
          value: group,
          label: group === 'all' ? t('全部分组') : group,
          tagCount: ratio,
        };
      }),
    [categoryProps.groupRatio, categoryProps.usableGroup, groupCountModels, t],
  );

  const quotaTypeItems = React.useMemo(
    () => [
      { value: 'all', label: t('全部类型'), tagCount: quotaTypeModels.length },
      {
        value: 0,
        label: t('按量计费'),
        tagCount: quotaTypeModels.filter((model) => model.quota_type === 0).length,
      },
      {
        value: 1,
        label: t('按次计费'),
        tagCount: quotaTypeModels.filter((model) => model.quota_type === 1).length,
      },
    ],
    [quotaTypeModels, t],
  );

  const tagItems = React.useMemo(() => {
    const tagSet = new Set();
    (categoryProps.models || []).forEach((model) => {
      if (!model.tags) return;
      model.tags
        .split(/[,;|]+/)
        .map((tag) => tag.trim())
        .filter(Boolean)
        .forEach((tag) => tagSet.add(tag.toLowerCase()));
    });

    const getTagCount = (tag) => {
      if (tag === 'all') return tagModels.length;
      return tagModels.filter((model) =>
        model.tags
          ?.toLowerCase()
          .split(/[,;|]+/)
          .map((item) => item.trim())
          .includes(tag),
      ).length;
    };

    return [
      { value: 'all', label: t('全部标签'), tagCount: getTagCount('all') },
      ...Array.from(tagSet)
        .sort((a, b) => a.localeCompare(b))
        .map((tag) => ({
          value: tag,
          label: tag,
          tagCount: getTagCount(tag),
        })),
    ];
  }, [categoryProps.models, t, tagModels]);

  const endpointTypeItems = React.useMemo(() => {
    const endpointTypes = new Set();
    (categoryProps.models || []).forEach((model) => {
      if (Array.isArray(model.supported_endpoint_types)) {
        model.supported_endpoint_types.forEach((endpoint) => endpointTypes.add(endpoint));
      }
    });

    return [
      { value: 'all', label: t('全部端点'), tagCount: endpointTypeModels.length },
      ...Array.from(endpointTypes)
        .sort()
        .map((endpointType) => ({
          value: endpointType,
          label: endpointType,
          tagCount: endpointTypeModels.filter((model) =>
            model.supported_endpoint_types?.includes(endpointType),
          ).length,
        })),
    ];
  }, [categoryProps.models, endpointTypeModels, t]);

  return (
    <div className='pricing-market-filters'>
      <div className='pricing-market-filters-header'>
        <div className='pricing-market-filters-title'>{t('筛选模型')}</div>
        <div className='pricing-market-filters-actions'>
          <Button
            theme='borderless'
            type='tertiary'
            onClick={handleResetFilters}
            className='pricing-market-reset-button'
          >
            {t('重置')}
          </Button>
          <Button
            theme='borderless'
            type='tertiary'
            onClick={() => setExpanded((open) => !open)}
            className='pricing-market-reset-button'
          >
            {expanded ? t('收起') : t('展开')}
          </Button>
        </div>
      </div>

      {expanded && (
        <div className='pricing-market-filters-body'>
          <PricingFilterRow
            label={t('供应商')}
            items={vendorItems}
            activeValue={filterVendor}
            onChange={setFilterVendor}
          />

          <PricingFilterRow
            label={t('可用令牌分组')}
            items={groupItems}
            activeValue={filterGroup}
            onChange={handleGroupClick}
          />

          <PricingFilterRow
            label={t('计费类型')}
            items={quotaTypeItems}
            activeValue={filterQuotaType}
            onChange={setFilterQuotaType}
          />

          <PricingFilterRow
            label={t('标签')}
            items={tagItems}
            activeValue={filterTag}
            onChange={setFilterTag}
          />

          <PricingFilterRow
            label={t('端点类型')}
            items={endpointTypeItems}
            activeValue={filterEndpointType}
            onChange={setFilterEndpointType}
          />
        </div>
      )}
    </div>
  );
};

export default PricingSidebar;
