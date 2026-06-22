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
import SkeletonWrapper from '../components/SkeletonWrapper';

const Navigation = ({
  mainNavLinks,
  isMobile,
  isLoading,
  userState,
  pricingRequireAuth,
  currentPath,
}) => {
  const isActiveLink = (link) => {
    if (link.isExternal) return false;

    if (link.itemKey === 'home') {
      return currentPath === '/';
    }
    if (link.itemKey === 'console') {
      return currentPath === '/console' || currentPath.startsWith('/console/');
    }

    return currentPath === link.to;
  };

  const renderNavLinks = () => {
    const baseClasses = `relative transition-colors ${isMobile ? 'p-1' : ''}`;
    const activeClasses = 'xmodel-headerbar-nav-active font-medium';
    const inactiveClasses = 'xmodel-headerbar-nav-link';

    const getLinkClasses = (isActive) =>
      `${baseClasses} ${isActive ? activeClasses : inactiveClasses}`;

    return mainNavLinks.map((link) => {
      const isActive = isActiveLink(link);
      const linkContent = (
        <>
          <span>{link.text}</span>
          {isActive && (
            <span className='absolute -bottom-1 left-0 right-0 mx-auto h-0.5 w-6 rounded-full bg-gradient-to-r from-brand-blue to-brand-purple' />
          )}
        </>
      );

      if (link.isExternal) {
        return (
          <a
            key={link.itemKey}
            href={link.externalLink}
            target='_blank'
            rel='noopener noreferrer'
            className={getLinkClasses(isActive)}
          >
            {linkContent}
          </a>
        );
      }

      let targetPath = link.to;
      if (link.itemKey === 'console' && !userState.user) {
        targetPath = '/login';
      }
      if (link.itemKey === 'pricing' && pricingRequireAuth && !userState.user) {
        targetPath = '/login';
      }

      return (
        <Link
          key={link.itemKey}
          to={targetPath}
          className={getLinkClasses(isActive)}
        >
          {linkContent}
        </Link>
      );
    });
  };

  return (
    <nav className='flex flex-1 items-center gap-[2.625rem] ml-6 mr-2 md:ml-8 md:mr-4 overflow-visible whitespace-nowrap text-base'>
      <SkeletonWrapper
        loading={isLoading}
        type='navigation'
        count={4}
        width={60}
        height={16}
        isMobile={isMobile}
      >
        {renderNavLinks()}
      </SkeletonWrapper>
    </nav>
  );
};

export default Navigation;
