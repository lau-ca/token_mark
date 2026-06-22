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

import React, { useEffect, useMemo, useState } from 'react';
import {
  Button,
  Empty,
  Form,
  Modal,
  Space,
  TabPane,
  Tabs,
  Tag,
  Typography,
} from '@douyinfe/semi-ui';
import { IconPlus, IconUserGroup } from '@douyinfe/semi-icons';
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
import {
  displayAmountToQuota,
  quotaToDisplayAmount,
} from '../../helpers/quota';

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

const Distribution = () => {
  const { t } = useTranslation();
  const [loading, setLoading] = useState(false);
  const [summary, setSummary] = useState({});
  const [inviteUsers, setInviteUsers] = useState([]);
  const [withdrawals, setWithdrawals] = useState([]);
  const [withdrawalPage, setWithdrawalPage] = useState({
    page: 1,
    page_size: 10,
    total: 0,
  });
  const [showModal, setShowModal] = useState(false);
  const [formApi, setFormApi] = useState(null);
  const [submitLoading, setSubmitLoading] = useState(false);
  const [withdrawMethod, setWithdrawMethod] = useState('platform');

  const loadData = async (
    page = withdrawalPage.page,
    pageSize = withdrawalPage.page_size,
  ) => {
    setLoading(true);
    try {
      const res = await API.get(
        `/api/user/distribution?p=${page}&page_size=${pageSize}`,
      );
      const { success, message, data } = res.data;
      if (!success) {
        showError(message);
        return;
      }
      setSummary(data.summary || {});
      setInviteUsers(
        (data.invite_users || []).map((item) => ({ ...item, key: item.id })),
      );
      const withdrawalData = data.withdrawals || {};
      setWithdrawals(
        (withdrawalData.items || []).map((item) => ({ ...item, key: item.id })),
      );
      setWithdrawalPage({
        page: withdrawalData.page || page,
        page_size: withdrawalData.page_size || pageSize,
        total: withdrawalData.total || 0,
      });
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    loadData(1, withdrawalPage.page_size);
  }, []);

  const handleSubmit = async () => {
    const values = formApi?.getValues() || {};
    const amount = displayAmountToQuota(values.amount);
    setSubmitLoading(true);
    try {
      const res = await API.post('/api/user/distribution/withdrawals', {
        amount,
        method: values.method,
        account: values.account || '',
        real_name: values.real_name || '',
      });
      const { success, message } = res.data;
      if (!success) {
        showError(message);
        return;
      }
      showSuccess(t('申请已提交'));
      setShowModal(false);
      await loadData(1, withdrawalPage.page_size);
    } finally {
      setSubmitLoading(false);
    }
  };

  const inviteColumns = useMemo(
    () => [
      {
        title: t('用户名'),
        dataIndex: 'username',
      },
      {
        title: t('剩余额度/总额度'),
        dataIndex: 'quota',
        render: (text, record) =>
          `${renderQuota(record.quota)} / ${renderQuota(record.recharge_quota || 0)}`,
      },
      {
        title: t('创建时间'),
        dataIndex: 'created_at',
        render: (text) => (text ? timestamp2string(text) : '-'),
      },
      {
        title: t('最后登录'),
        dataIndex: 'last_login_at',
        render: (text) => (text ? timestamp2string(text) : '-'),
      },
    ],
    [t],
  );

  const withdrawalColumns = useMemo(
    () => [
      {
        title: t('申请时间'),
        dataIndex: 'created_at',
        render: (text) => (text ? timestamp2string(text) : '-'),
      },
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
        title: t('处理时间'),
        dataIndex: 'handled_at',
        render: (text) => (text ? timestamp2string(text) : '-'),
      },
      {
        title: t('备注'),
        dataIndex: 'remark',
        render: (text) => text || '-',
      },
    ],
    [t],
  );

  const stats = [
    { label: t('邀请用户'), value: summary.invite_count || 0 },
    { label: t('分销累计额度'), value: renderQuota(summary.total_amount || 0) },
    {
      label: t('已提现额度'),
      value: renderQuota(summary.approved_amount || 0),
    },
    { label: t('待审核额度'), value: renderQuota(summary.pending_amount || 0) },
    {
      label: t('可提现额度'),
      value: renderQuota(summary.available_amount || 0),
    },
  ];

  return (
    <div className='mt-[60px] px-2'>
      <CardPro
        type='type1'
        descriptionArea={
          <div className='flex flex-col md:flex-row justify-between items-start md:items-center gap-3 w-full'>
            <div className='flex items-center text-blue-500'>
              <IconUserGroup className='mr-2' />
              <Text>{t('分销管理')}</Text>
            </div>
            <div className='grid grid-cols-2 md:grid-cols-5 gap-2 w-full md:w-auto'>
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
        actionsArea={
          <div className='flex justify-end w-full'>
            <Button
              type='primary'
              icon={<IconPlus />}
              disabled={(summary.available_amount || 0) <= 0}
              onClick={() => setShowModal(true)}
            >
              {t('申请提现')}
            </Button>
          </div>
        }
        t={t}
      >
        <Tabs type='line'>
          <TabPane itemKey='users' tab={t('邀请用户')}>
            <CardTable
              columns={inviteColumns}
              dataSource={inviteUsers}
              loading={loading}
              pagination={false}
              empty={
                <Empty
                  description={t('暂无邀请用户')}
                  style={{ padding: 30 }}
                />
              }
              rowKey='id'
            />
          </TabPane>
          <TabPane itemKey='withdrawals' tab={t('提现记录')}>
            <CardTable
              columns={withdrawalColumns}
              dataSource={withdrawals}
              loading={loading}
              rowKey='id'
              pagination={{
                currentPage: withdrawalPage.page,
                pageSize: withdrawalPage.page_size,
                total: withdrawalPage.total,
                onPageChange: (page) =>
                  loadData(page, withdrawalPage.page_size),
                onPageSizeChange: (pageSize) => loadData(1, pageSize),
              }}
            />
          </TabPane>
        </Tabs>
      </CardPro>

      <Modal
        title={t('申请提现')}
        visible={showModal}
        onCancel={() => setShowModal(false)}
        onOk={handleSubmit}
        confirmLoading={submitLoading}
      >
        <Form
          getFormApi={setFormApi}
          initValues={{
            method: 'platform',
            amount: quotaToDisplayAmount(summary.available_amount || 0),
          }}
          onValueChange={(values) =>
            setWithdrawMethod(values.method || 'platform')
          }
        >
          <Form.InputNumber
            field='amount'
            label={t('提现额度')}
            min={0}
            precision={2}
          />
          <Form.Select
            field='method'
            label={t('提现方式')}
            optionList={[
              { label: t('平台'), value: 'platform' },
              { label: t('支付宝'), value: 'alipay' },
            ]}
          />
          {withdrawMethod === 'alipay' && (
            <>
              <Form.Input field='account' label={t('支付宝账号')} />
              <Form.Input field='real_name' label={t('收款姓名')} />
            </>
          )}
          <Space vertical align='start'>
            <Text type='tertiary'>
              {t('可提现额度')}: {renderQuota(summary.available_amount || 0)}
            </Text>
          </Space>
        </Form>
      </Modal>
    </div>
  );
};

export default Distribution;
