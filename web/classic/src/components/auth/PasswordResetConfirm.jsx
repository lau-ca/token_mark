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

import React, { useEffect, useState } from 'react';
import {
  API,
  copy,
  showError,
  showNotice,
  getLogo,
  getSystemName,
} from '../../helpers';
import { useSearchParams, Link } from 'react-router-dom';
import { Button, Form, Typography, Banner } from '@douyinfe/semi-ui';
import { IconCopy } from '@douyinfe/semi-icons';
import { useTranslation } from 'react-i18next';
import { ArrowRight } from 'lucide-react';
import AuthPageFrame from './AuthPageFrame';

const { Text } = Typography;

const PasswordResetConfirm = () => {
  const { t } = useTranslation();
  const [inputs, setInputs] = useState({
    email: '',
    token: '',
  });
  const { email, token } = inputs;
  const isValidResetLink = email && token;

  const [loading, setLoading] = useState(false);
  const [disableButton, setDisableButton] = useState(false);
  const [countdown, setCountdown] = useState(30);
  const [newPassword, setNewPassword] = useState('');
  const [searchParams, setSearchParams] = useSearchParams();
  const [formApi, setFormApi] = useState(null);

  const logo = getLogo();
  const systemName = getSystemName();

  useEffect(() => {
    let token = searchParams.get('token');
    let email = searchParams.get('email');
    setInputs({
      token: token || '',
      email: email || '',
    });
    if (formApi) {
      formApi.setValues({
        email: email || '',
        newPassword: newPassword || '',
      });
    }
  }, [searchParams, newPassword, formApi]);

  useEffect(() => {
    let countdownInterval = null;
    if (disableButton && countdown > 0) {
      countdownInterval = setInterval(() => {
        setCountdown(countdown - 1);
      }, 1000);
    } else if (countdown === 0) {
      setDisableButton(false);
      setCountdown(30);
    }
    return () => clearInterval(countdownInterval);
  }, [disableButton, countdown]);

  async function handleSubmit(e) {
    if (!email || !token) {
      showError(t('无效的重置链接，请重新发起密码重置请求'));
      return;
    }
    setDisableButton(true);
    setLoading(true);
    const res = await API.post(`/api/user/reset`, {
      email,
      token,
    });
    const { success, message } = res.data;
    if (success) {
      let password = res.data.data;
      setNewPassword(password);
      await copy(password);
      showNotice(`${t('密码已重置并已复制到剪贴板：')} ${password}`);
    } else {
      showError(message);
    }
    setLoading(false);
  }

  return (
    <AuthPageFrame
      logo={logo}
      systemName={systemName}
      title={t('密码重置确认')}
      subtitle={t(
        '确认邮箱重置链接后，系统会生成新密码并自动复制到剪贴板，请及时登录后修改。',
      )}
    >
      <div className='xmodel-auth-form-shell'>
        <div className='w-full'>
          <div className='xmodel-auth-card'>
            <div className='px-2 py-8'>
              {!isValidResetLink && (
                <Banner
                  type='danger'
                  description={t('无效的重置链接，请重新发起密码重置请求')}
                  className='mb-4 !rounded-lg'
                  closeIcon={null}
                />
              )}
              <Form
                getFormApi={(api) => setFormApi(api)}
                initValues={{
                  email: email || '',
                  newPassword: newPassword || '',
                }}
                className='space-y-3'
              >
                <Form.Input
                  field='email'
                  name='email'
                  disabled={true}
                  placeholder={email ? '' : t('等待获取邮箱信息...')}
                />

                {newPassword && (
                  <Form.Input
                    field='newPassword'
                    name='newPassword'
                    disabled={true}
                    suffix={
                      <Button
                        icon={<IconCopy />}
                        type='tertiary'
                        theme='borderless'
                        onClick={async () => {
                          await copy(newPassword);
                          showNotice(
                            `${t('密码已复制到剪贴板：')} ${newPassword}`,
                          );
                        }}
                      >
                        {t('复制')}
                      </Button>
                    }
                  />
                )}

                <div className='space-y-2 pt-2'>
                  <Button
                    theme='solid'
                    className='w-full rounded-xl'
                    type='primary'
                    htmlType='submit'
                    onClick={handleSubmit}
                    loading={loading}
                    disabled={disableButton || newPassword || !isValidResetLink}
                  >
                    {newPassword ? t('密码重置完成') : t('确认重置密码')}
                    {!newPassword && (
                      <ArrowRight className='xmodel-auth-submit-icon' />
                    )}
                  </Button>
                </div>
              </Form>

              <div className='mt-6 text-center text-sm'>
                <Text>
                  <Link
                    to='/login'
                    className='text-blue-600 hover:text-blue-800 font-medium'
                  >
                    {t('返回登录')}
                  </Link>
                </Text>
              </div>
            </div>
          </div>
        </div>
      </div>
    </AuthPageFrame>
  );
};

export default PasswordResetConfirm;
