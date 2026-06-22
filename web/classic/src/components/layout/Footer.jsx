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
import { Link } from 'react-router-dom';
import { getLogo, getSystemName } from '../../helpers';

const preventPlaceholderNavigation = (event) => {
  event.preventDefault();
};

const FooterLink = ({ children, to }) =>
  to ? (
    <Link to={to}>{children}</Link>
  ) : (
    <a href='#' onClick={preventPlaceholderNavigation}>
      {children}
    </a>
  );

const FooterBar = () => {
  const systemName = getSystemName();
  const logo = getLogo();
  const currentYear = new Date().getFullYear();

  return (
    <footer className='xmodel-footer'>
      <div className='xmodel-footer-inner'>
        <div className='xmodel-footer-brand'>
          <img src={logo} alt={systemName} />
          <div>
            <strong>{systemName}</strong>
            <span>© {currentYear} AI 大模型聚合平台</span>
          </div>
        </div>

        <div className='xmodel-footer-links'>
          <div>
            <strong>产品</strong>
            <FooterLink to='/gateway#platform-capabilities'>
              平台能力
            </FooterLink>
          </div>
          <div>
            <strong>问题</strong>
            <FooterLink>问题反馈</FooterLink>
          </div>
          <div>
            <strong>法律</strong>
            <span className='xmodel-footer-legal-links'>
              <FooterLink to='/user-agreement'>用户协议</FooterLink>
              <span>/</span>
              <FooterLink to='/privacy-policy'>隐私政策</FooterLink>
            </span>
          </div>
        </div>
      </div>
    </footer>
  );
};

export default FooterBar;
