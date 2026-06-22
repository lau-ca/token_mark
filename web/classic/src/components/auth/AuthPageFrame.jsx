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
import { Globe2, ShieldCheck, Sparkles, Zap } from 'lucide-react';
import loginBg from '../../assets/login-bg.png';

const authFeatures = [
  {
    title: '主流模型',
    desc: 'GPT、Claude、Gemini、DeepSeek 一站接入',
    icon: Globe2,
  },
  {
    title: '极速稳定调用',
    desc: '多地域节点 · 智能路由 · 99.9% SLA',
    icon: Zap,
  },
  {
    title: '企业级安全',
    desc: '统一计费 · 权限管控 · 审计日志',
    icon: ShieldCheck,
  },
];

const AuthPageFrame = ({
  logo,
  systemName,
  title,
  subtitle,
  children,
  turnstile,
}) => {
  return (
    <div className='xmodel-auth-page'>
      <div
        className='xmodel-auth-image-bg'
        style={{ backgroundImage: `url(${loginBg})` }}
      />
      <div className='xmodel-auth-grid-bg' />
      <div className='xmodel-auth-radial-bg' />
      <div className='xmodel-auth-light-glow xmodel-auth-light-glow-blue' />
      <div className='xmodel-auth-light-glow xmodel-auth-light-glow-purple' />
      <div className='xmodel-auth-dark-radial' />
      <div className='xmodel-auth-dark-glow xmodel-auth-dark-glow-blue' />
      <div className='xmodel-auth-dark-glow xmodel-auth-dark-glow-purple' />

      <main className='xmodel-auth-main'>
        <section className='xmodel-auth-copy'>
          <div className='xmodel-auth-eyebrow'>
            <Sparkles />
            <span>Welcome to Xmodel</span>
          </div>
          <h1>
            <span>Xmodel</span>
          </h1>
          <p>{subtitle}</p>

          <div className='xmodel-auth-feature-list'>
            {authFeatures.map((feature) => {
              const FeatureIcon = feature.icon;
              return (
                <div className='xmodel-auth-feature' key={feature.title}>
                  <span>
                    <FeatureIcon />
                  </span>
                  <div>
                    <strong>{feature.title}</strong>
                    <small>{feature.desc}</small>
                  </div>
                </div>
              );
            })}
          </div>
        </section>

        <section className='xmodel-auth-panel'>
          <div className='xmodel-auth-panel-glow xmodel-auth-panel-glow-blue' />
          <div className='xmodel-auth-panel-glow xmodel-auth-panel-glow-purple' />
          <div className='xmodel-auth-card-heading'>
            <span>
              <Sparkles />
            </span>
            <h2>{title}</h2>
          </div>
          {children}
          {turnstile}
        </section>
      </main>
    </div>
  );
};

export default AuthPageFrame;
