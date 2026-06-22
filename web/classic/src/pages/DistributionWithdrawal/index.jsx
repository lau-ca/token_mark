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

import React, { useEffect, useMemo, useRef, useState } from 'react';
import {
  Button,
  Empty,
  Form,
  Modal,
  Space,
  Tag,
  Typography,
} from '@douyinfe/semi-ui';
import { IconSearch, IconUserCardVideo } from '@douyinfe/semi-icons';
import { useTranslation } from 'react-i18next';
import CardPro from '../../components/common/ui/CardPro';
import CardTable from '../../components/common/ui/CardTable';
import {
  API,
  renderQuota,
  showError,
  showSuccess,
  timestamp2string,
} from '../../helpers';

const { Text } = Typography;

const METHOD_TEXT = {
  alipay: '支付宝',
  platform: '平台',
};

const STATUS_COLOR = {
  pending: 'orange',
  approved: 'green',
  rejected: 'red',
};

const STATUS_TEXT = {
  pending: '待审核',
  approved: '已通过',
  rejected: '已拒绝',
};

const DistributionWithdrawal = () => {
  const { t } = useTranslation();
  const formApiRef = useRef(null);
  const [loading, setLoading] = useState(false);
  const [summary, setSummary] = useState({});
  const [withdrawals, setWithdrawals] = useState([]);
  const [pageInfo, setPageInfo] = useState({
    page: 1,
    page_size: 10,
    total: 0,
  });
  const [currentRecord, setCurrentRecord] = useState(null);
  const [approveModalVisible, setApproveModalVisible] = useState(false);
  const [rejectModalVisible, setRejectModalVisible] = useState(false);
  const [handleLoading, setHandleLoading] = useState(false);
  const [remark, setRemark] = useState('');

  const getFilters = () => formApiRef.current?.getValues() || {};

  const loadData = async (
    page = pageInfo.page,
    pageSize = pageInfo.page_size,
  ) => {
    setLoading(true);
    try {
      const filters = getFilters();
      const params = new URLSearchParams({
        p: String(page),
        page_size: String(pageSize),
      });
      if (filters.status) params.set('status', filters.status);
      if (filters.method) params.set('method', filters.method);
      if (filters.username) params.set('username', filters.username);

      const res = await API.get(
        `/api/user/distribution/withdrawals?${params.toString()}`,
      );
      const { success, message, data } = res.data;
      if (!success) {
        showError(message);
        return;
      }

      setSummary(data.summary || {});
      const withdrawalData = data.withdrawals || {};
      setWithdrawals(
        (withdrawalData.items || []).map((item) => ({ ...item, key: item.id })),
      );
      setPageInfo({
        page: withdrawalData.page || page,
        page_size: withdrawalData.page_size || pageSize,
        total: withdrawalData.total || 0,
      });
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    loadData(1, pageInfo.page_size);
  }, []);

  const openHandleModal = (record, approved) => {
    setCurrentRecord(record);
    setRemark('');
    if (approved) {
      setApproveModalVisible(true);
    } else {
      setRejectModalVisible(true);
    }
  };

  const submitHandle = async (approved) => {
    if (!currentRecord) return;
    setHandleLoading(true);
    try {
      const res = await API.post(
        `/api/user/distribution/withdrawals/${currentRecord.id}/handle`,
        {
          approved,
          remark,
        },
      );
      const { success, message } = res.data;
      if (!success) {
        showError(message);
        return;
      }
      showSuccess(t('操作成功完成！'));
      setApproveModalVisible(false);
      setRejectModalVisible(false);
      await loadData(pageInfo.page, pageInfo.page_size);
    } finally {
      setHandleLoading(false);
    }
  };

  const columns = useMemo(
    () => [
      { title: 'ID', dataIndex: 'id' },
      { title: t('用户名'), dataIndex: 'username' },
      {
        title: t('提现方式'),
        dataIndex: 'method',
        render: (text) => t(METHOD_TEXT[text] || text),
      },
      {
        title: t('提现额度'),
        dataIndex: 'amount',
        render: (text) => renderQuota(text),
      },
      {
        title: t('状态'),
        dataIndex: 'status',
        render: (text) => (
          <Tag color={STATUS_COLOR[text] || 'grey'} shape='circle'>
            {t(STATUS_TEXT[text] || text)}
          </Tag>
        ),
      },
      {
        title: t('收款信息'),
        dataIndex: 'account',
        render: (text, record) => {
          if (record.method === 'platform') return t('平台余额');
          return text ? `${record.real_name} / ${text}` : '-';
        },
      },
      {
        title: t('申请时间'),
        dataIndex: 'created_at',
        render: (text) => (text ? timestamp2string(text) : '-'),
      },
      {
        title: t('处理时间'),
        dataIndex: 'handled_at',
        render: (text) => (text ? timestamp2string(text) : '-'),
      },
      {
        title: '',
        dataIndex: 'operate',
        fixed: 'right',
        render: (text, record) => {
          if (record.status !== 'pending') return null;
          return (
            <Space>
              <Button
                type='primary'
                size='small'
                onClick={() => openHandleModal(record, true)}
              >
                {t('通过')}
              </Button>
              <Button
                type='danger'
                size='small'
                onClick={() => openHandleModal(record, false)}
              >
                {t('拒绝')}
              </Button>
            </Space>
          );
        },
      },
    ],
    [t],
  );

  const stats = [
    { label: t('待审核笔数'), value: summary.pending_count || 0 },
    { label: t('待审核额度'), value: renderQuota(summary.pending_amount || 0) },
    {
      label: t('已通过额度'),
      value: renderQuota(summary.approved_amount || 0),
    },
    {
      label: t('已拒绝额度'),
      value: renderQuota(summary.rejected_amount || 0),
    },
  ];

  const filters = (
    <Form
      getFormApi={(api) => {
        formApiRef.current = api;
      }}
      onSubmit={() => loadData(1, pageInfo.page_size)}
      allowEmpty
      layout='horizontal'
      className='w-full'
    >
      <div className='flex flex-col md:flex-row items-center gap-2 w-full'>
        <Form.Input
          field='username'
          prefix={<IconSearch />}
          placeholder={t('用户名')}
          showClear
          pure
          size='small'
          className='w-full md:w-48'
        />
        <Form.Select
          field='status'
          placeholder={t('状态')}
          showClear
          pure
          size='small'
          className='w-full md:w-40'
          optionList={[
            { label: t('待审核'), value: 'pending' },
            { label: t('已通过'), value: 'approved' },
            { label: t('已拒绝'), value: 'rejected' },
          ]}
        />
        <Form.Select
          field='method'
          placeholder={t('提现方式')}
          showClear
          pure
          size='small'
          className='w-full md:w-40'
          optionList={[
            { label: t('平台'), value: 'platform' },
            { label: t('支付宝'), value: 'alipay' },
          ]}
        />
        <Button
          type='tertiary'
          htmlType='submit'
          loading={loading}
          size='small'
        >
          {t('查询')}
        </Button>
        <Button
          type='tertiary'
          size='small'
          onClick={() => {
            formApiRef.current?.reset();
            setTimeout(() => loadData(1, pageInfo.page_size), 100);
          }}
        >
          {t('重置')}
        </Button>
      </div>
    </Form>
  );

  const renderHandleModal = (approved) => (
    <Modal
      title={approved ? t('通过提现申请') : t('拒绝提现申请')}
      visible={approved ? approveModalVisible : rejectModalVisible}
      onCancel={() => {
        setApproveModalVisible(false);
        setRejectModalVisible(false);
      }}
      onOk={() => submitHandle(approved)}
      confirmLoading={handleLoading}
    >
      {currentRecord && (
        <Space vertical align='start' style={{ width: '100%' }}>
          <Text>
            {t('用户名')}: {currentRecord.username}
          </Text>
          <Text>
            {t('提现方式')}:{' '}
            {t(METHOD_TEXT[currentRecord.method] || currentRecord.method)}
          </Text>
          <Text>
            {t('提现额度')}: {renderQuota(currentRecord.amount)}
          </Text>
          <Text>
            {t('收款信息')}:{' '}
            {currentRecord.method === 'platform'
              ? t('平台余额')
              : `${currentRecord.real_name || '-'} / ${currentRecord.account || '-'}`}
          </Text>
          <Form style={{ width: '100%' }}>
            <Form.TextArea
              field='remark'
              label={t('备注')}
              value={remark}
              onChange={setRemark}
              autosize
            />
          </Form>
        </Space>
      )}
    </Modal>
  );

  return (
    <div className='mt-[60px] px-2'>
      <CardPro
        type='type1'
        descriptionArea={
          <div className='flex flex-col md:flex-row justify-between items-start md:items-center gap-3 w-full'>
            <div className='flex items-center text-blue-500'>
              <IconUserCardVideo className='mr-2' />
              <Text>{t('分销提现')}</Text>
            </div>
            <div className='grid grid-cols-2 md:grid-cols-4 gap-2 w-full md:w-auto'>
              {stats.map((item) => (
                <div
                  key={item.label}
                  className='px-3 py-2 rounded-lg border'
                  style={{ borderColor: 'var(--semi-color-border)' }}
                >
                  <div className='text-xs text-semi-color-text-2'>
                    {item.label}
                  </div>
                  <div className='text-sm font-semibold text-semi-color-text-0 mt-1'>
                    {item.value}
                  </div>
                </div>
              ))}
            </div>
          </div>
        }
        actionsArea={filters}
        t={t}
      >
        <CardTable
          columns={columns}
          dataSource={withdrawals}
          loading={loading}
          rowKey='id'
          scroll={{ x: 'max-content' }}
          pagination={{
            currentPage: pageInfo.page,
            pageSize: pageInfo.page_size,
            total: pageInfo.total,
            onPageChange: (page) => loadData(page, pageInfo.page_size),
            onPageSizeChange: (pageSize) => loadData(1, pageSize),
          }}
          empty={
            <Empty description={t('暂无提现申请')} style={{ padding: 30 }} />
          }
        />
      </CardPro>
      {renderHandleModal(true)}
      {renderHandleModal(false)}
    </div>
  );
};

export default DistributionWithdrawal;
