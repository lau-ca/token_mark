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

import React, { useRef } from 'react';
import { Link } from 'react-router-dom';
import { Avatar, Button, Dropdown, Typography } from '@douyinfe/semi-ui';
import { ArrowRight, ChevronDown } from 'lucide-react';
import {
  IconExit,
  IconUserSetting,
  IconCreditCard,
  IconKey,
} from '@douyinfe/semi-icons';
import { stringToColor } from '../../../helpers';
import SkeletonWrapper from '../components/SkeletonWrapper';

const UserArea = ({
  userState,
  isLoading,
  isMobile,
  isSelfUseMode,
  logout,
  navigate,
  t,
}) => {
  const dropdownRef = useRef(null);
  if (isLoading) {
    return (
      <SkeletonWrapper
        loading={true}
        type='userArea'
        width={50}
        isMobile={isMobile}
      />
    );
  }

  if (userState.user) {
    return (
      <div className='relative' ref={dropdownRef}>
        <Dropdown
          position='bottomRight'
          getPopupContainer={() => dropdownRef.current}
          render={
            <Dropdown.Menu className='xmodel-headerbar-dropdown'>
              <Dropdown.Item
                onClick={() => {
                  navigate('/console/personal');
                }}
                className='xmodel-headerbar-dropdown-item !px-3 !py-1.5 !text-sm'
              >
                <div className='flex items-center gap-2'>
                  <IconUserSetting
                    size='small'
                    className='xmodel-headerbar-dropdown-icon'
                  />
                  <span>{t('个人设置')}</span>
                </div>
              </Dropdown.Item>
              <Dropdown.Item
                onClick={() => {
                  navigate('/console/token');
                }}
                className='xmodel-headerbar-dropdown-item !px-3 !py-1.5 !text-sm'
              >
                <div className='flex items-center gap-2'>
                  <IconKey
                    size='small'
                    className='xmodel-headerbar-dropdown-icon'
                  />
                  <span>{t('令牌管理')}</span>
                </div>
              </Dropdown.Item>
              <Dropdown.Item
                onClick={() => {
                  navigate('/console/topup');
                }}
                className='xmodel-headerbar-dropdown-item !px-3 !py-1.5 !text-sm'
              >
                <div className='flex items-center gap-2'>
                  <IconCreditCard
                    size='small'
                    className='xmodel-headerbar-dropdown-icon'
                  />
                  <span>{t('钱包管理')}</span>
                </div>
              </Dropdown.Item>
              <Dropdown.Item
                onClick={logout}
                className='xmodel-headerbar-dropdown-item is-danger !px-3 !py-1.5 !text-sm'
              >
                <div className='flex items-center gap-2'>
                  <IconExit
                    size='small'
                    className='xmodel-headerbar-dropdown-icon'
                  />
                  <span>{t('退出')}</span>
                </div>
              </Dropdown.Item>
            </Dropdown.Menu>
          }
        >
          <Button
            theme='borderless'
            type='tertiary'
            className='xmodel-headerbar-user-button flex items-center gap-1.5 !p-1 !rounded-full'
          >
            <Avatar
              size='extra-small'
              color={stringToColor(userState.user.username)}
              className='mr-1'
            >
              {userState.user.username[0].toUpperCase()}
            </Avatar>
            <span className='hidden md:inline'>
              <Typography.Text className='!text-xs !font-medium mr-1'>
                {userState.user.username}
              </Typography.Text>
            </span>
            <ChevronDown
              size={14}
              className='xmodel-headerbar-user-chevron text-xs'
            />
          </Button>
        </Dropdown>
      </div>
    );
  } else {
    return (
      <div className='flex items-center'>
        <Link to='/login' className='flex'>
          <Button
            theme='solid'
            type='primary'
            className='rounded-full bg-gradient-to-r from-brand-blue to-brand-purple text-white hover:opacity-90 border-0 shadow-[0_0_24px_-4px_hsl(var(--brand-blue)/0.6)]'
          >
            <span className='hidden sm:inline'>{t('立即登录')}</span>
            <span className='sm:hidden'>{t('登录')}</span>
            <ArrowRight className='w-4 h-4 ml-1' />
          </Button>
        </Link>
      </div>
    );
  }
};

export default UserArea;
