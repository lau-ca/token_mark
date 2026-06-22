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
import { API, showError } from '../../helpers';
import { marked } from 'marked';
import { Modal } from '@douyinfe/semi-ui';
import { useTranslation } from 'react-i18next';
import {
  ArrowRight,
  Briefcase,
  Code2,
  Globe2,
  LifeBuoy,
  Plug,
  ShieldCheck,
  Sparkles,
  Target,
} from 'lucide-react';
import wechatGroupQr from '../../assets/xmodel-wechat-group-qr.jpg';

const advantages = [
  {
    icon: Globe2,
    title: 'AI 模型支持',
    desc: '接入 GPT、Claude、Gemini、DeepSeek 等主流大模型,持续扩展。',
  },
  {
    icon: Plug,
    title: '统一 API 接口',
    desc: '完全兼容 OpenAI 协议,一行代码即可在不同模型之间切换。',
  },
  {
    icon: ShieldCheck,
    title: '稳定高可用架构',
    desc: '多地域节点 + 智能路由,99.9% SLA 保障线上业务稳定运行。',
  },
  {
    icon: Code2,
    title: '开发者友好',
    desc: '完整文档、示例与 SDK,快速上手,5 分钟完成第一次接入。',
  },
];

const contacts = [
  {
    icon: Briefcase,
    label: '商务合作',
    value: 'jiuyuxinyun@jiuyu.tech',
    href: 'mailto:jiuyuxinyun@jiuyu.tech',
    qr: wechatGroupQr,
  },
  {
    icon: LifeBuoy,
    label: '技术支持',
    value: 'jiuyuxinyun@jiuyu.tech',
    href: 'mailto:jiuyuxinyun@jiuyu.tech',
    qr: wechatGroupQr,
  },
];

const stats = [
  { value: '20+', label: '接入主流模型' },
  { value: '99.9%', label: '服务可用性' },
  { value: '10ms', label: '平均路由延迟' },
  { value: '10k+', label: '活跃开发者' },
];

const hoverTransition = 'transition-all duration-700';
const hoverOpacityTransition = 'transition-opacity duration-700';

const AboutCard = ({ className = '', children }) => (
  <div
    className={`rounded-xl border border-white/10 bg-card/60 backdrop-blur ${className}`}
  >
    {children}
  </div>
);

const AboutUsContent = () => {
  const [activeQr, setActiveQr] = useState(null);

  return (
    <main className='min-h-screen bg-background text-foreground'>
      <section className='pt-32 pb-20 relative overflow-hidden'>
        <div className='absolute inset-0 bg-hero-radial opacity-70' />
        <div className='absolute top-20 left-1/2 -translate-x-1/2 w-[700px] h-[400px] rounded-full bg-brand-purple/10 blur-[140px] pointer-events-none' />
        <div className='relative container mx-auto px-6 text-center'>
          <div className='inline-flex items-center gap-2 px-3 py-1 rounded-full border border-white/10 bg-card/60 backdrop-blur text-xs text-muted-foreground mb-6'>
            <Sparkles className='w-3.5 h-3.5 text-brand-blue' />
            <span>About Xmodel</span>
          </div>
          <h1 className='text-4xl md:text-6xl font-semibold text-architectural mb-6'>
            关于 <span className='text-gradient-brand'>Xmodel</span>
          </h1>
          <p className='text-muted-foreground text-lg max-w-2xl mx-auto'>
            构建 AI 模型接入与 AI 创作工具平台,让开发者和企业更轻松地使用 AI
            能力。
          </p>
        </div>
      </section>

      <section className='pt-16 pb-2'>
        <div className='container mx-auto px-4 sm:px-6'>
          <div className='text-center'>
            <div className='text-xs uppercase tracking-widest text-brand-blue mb-2'>
              Who We Are
            </div>
            <h2 className='text-3xl md:text-4xl font-semibold text-foreground'>
              我们是谁
            </h2>
            <div className='mx-auto mt-4 max-w-3xl space-y-2 text-muted-foreground leading-relaxed'>
              <p>
                Xmodel 是一个专注于 AI 基础设施与 AI
                工具平台的团队,致力于帮助开发者和企业更简单地接入 AI
                模型,通过统一的 API 接口调用不同的 AI 服务;
              </p>
              <p>
                平台支持 GPT、Claude、Gemini、DeepSeek 等多种 AI
                模型,并提供稳定、高效的 API 接入服务。
              </p>
            </div>
          </div>
        </div>
      </section>

      <section className='pt-4 pb-12'>
        <div className='container mx-auto px-4 sm:px-6'>
          <div className='grid grid-cols-2 gap-4 md:grid-cols-4'>
            {stats.map((item) => (
              <AboutCard
                key={item.label}
                className='min-w-0 p-4 text-center sm:p-6'
              >
                <div className='whitespace-nowrap text-[clamp(1.75rem,9vw,2.25rem)] font-semibold leading-tight text-gradient-brand md:text-4xl'>
                  {item.value}
                </div>
                <div className='mt-2 text-xs tracking-wide text-muted-foreground'>
                  {item.label}
                </div>
              </AboutCard>
            ))}
          </div>
        </div>
      </section>

      <section className='py-16'>
        <div className='container mx-auto px-6'>
          <div className='text-center mb-10'>
            <div className='text-xs uppercase tracking-widest text-brand-purple mb-2'>
              Our Advantages
            </div>
            <h2 className='text-3xl md:text-4xl font-semibold text-foreground'>
              我们的优势
            </h2>
          </div>
          <div className='grid sm:grid-cols-2 gap-4'>
            {advantages.map((item) => {
              const Icon = item.icon;
              return (
                <AboutCard
                  key={item.title}
                  className={`group p-6 hover:border-brand-blue/50 hover:shadow-[0_0_30px_-12px_hsl(var(--brand-blue)/0.5)] ${hoverTransition}`}
                >
                  <div className='flex items-start gap-4'>
                    <span
                      className={`grid place-items-center w-11 h-11 rounded-xl bg-gradient-to-br from-brand-blue/20 to-brand-purple/20 text-brand-blue group-hover:from-brand-blue group-hover:to-brand-purple group-hover:text-white shrink-0 ${hoverTransition}`}
                    >
                      <Icon className='w-5 h-5' />
                    </span>
                    <div>
                      <div className='text-foreground font-medium text-lg'>
                        {item.title}
                      </div>
                      <p className='text-sm text-muted-foreground mt-1.5 leading-relaxed'>
                        {item.desc}
                      </p>
                    </div>
                  </div>
                </AboutCard>
              );
            })}
          </div>
        </div>
      </section>

      <section className='py-16'>
        <div className='container mx-auto px-6'>
          <AboutCard className='relative overflow-hidden p-10 md:p-14 text-center'>
            <div className='absolute -top-20 left-1/2 -translate-x-1/2 w-[500px] h-[300px] rounded-full bg-brand-blue/10 blur-[120px] pointer-events-none' />
            <div className='relative'>
              <Target className='w-8 h-8 text-brand-blue mx-auto mb-4' />
              <div className='text-xs uppercase tracking-widest text-brand-blue mb-3'>
                Our Mission
              </div>
              <h2 className='text-3xl md:text-4xl font-semibold text-foreground mb-6'>
                我们的使命
              </h2>
              <p className='text-muted-foreground text-lg leading-relaxed'>
                我们相信 AI 将改变未来的软件开发方式。Xmodel
                希望成为开发者和企业连接 AI 能力的重要基础设施,
                <br />让 AI 技术能够被更广泛、更轻松地应用。
              </p>
            </div>
          </AboutCard>
        </div>
      </section>

      <section className='py-16 pb-24'>
        <div className='container mx-auto px-6'>
          <div className='text-center mb-10'>
            <div className='text-xs uppercase tracking-widest text-brand-blue mb-2'>
              Contact
            </div>
            <h2 className='text-3xl md:text-4xl font-semibold text-foreground mb-3'>
              联系我们
            </h2>
            <p className='text-muted-foreground'>
              如果你希望合作或了解更多信息,可以通过以下方式联系我们。
            </p>
          </div>
          <div className='grid sm:grid-cols-2 gap-4'>
            {contacts.map((item) => {
              const Icon = item.icon;
              return (
                <a
                  key={item.label}
                  href={item.href}
                  className={`group block p-6 rounded-xl border border-white/10 bg-card/60 backdrop-blur hover:border-brand-blue/50 hover:shadow-[0_0_30px_-12px_hsl(var(--brand-blue)/0.5)] ${hoverTransition}`}
                >
                  <div className='flex items-center gap-5'>
                    <div className='flex-1 min-w-0'>
                      <span
                        className={`grid place-items-center w-10 h-10 rounded-lg bg-gradient-to-br from-brand-blue/20 to-brand-purple/20 text-brand-blue group-hover:from-brand-blue group-hover:to-brand-purple group-hover:text-white mb-3 ${hoverTransition}`}
                      >
                        <Icon className='w-4 h-4' />
                      </span>
                      <div className='text-foreground font-medium'>
                        {item.label}
                      </div>
                      <div className='text-sm text-muted-foreground mt-1 break-all'>
                        {item.value}
                      </div>
                    </div>
                    <img
                      src={item.qr}
                      alt={`${item.label} 二维码`}
                      onClick={(event) => {
                        event.preventDefault();
                        setActiveQr(item);
                      }}
                      className={`w-24 h-24 object-contain rounded-md bg-white p-1 shrink-0 opacity-80 ml-auto cursor-zoom-in hover:opacity-100 border border-border ${hoverOpacityTransition}`}
                    />
                  </div>
                </a>
              );
            })}
          </div>

          <div className='mt-12 flex flex-col md:flex-row items-center justify-between gap-4 p-6 rounded-2xl border border-white/10 bg-card/60 backdrop-blur'>
            <div>
              <div className='text-foreground font-medium'>
                准备好接入 Xmodel 了吗?
              </div>
              <div className='text-sm text-muted-foreground'>
                立即注册免费获取 API Key,开启你的智能体之旅。
              </div>
            </div>
            <a
              href='https://doc.xmodel.chat'
              target='_blank'
              rel='noopener noreferrer'
              className='hidden sm:block'
            >
              <span className='inline-flex h-10 items-center justify-center rounded-full border-0 bg-gradient-to-r from-brand-blue to-brand-purple px-5 text-sm font-medium text-white shadow-[0_0_24px_-6px_hsl(var(--brand-blue)/0.7)] transition-opacity hover:opacity-90'>
                查看 API 文档 <ArrowRight className='w-4 h-4 ml-1' />
              </span>
            </a>
          </div>
        </div>
      </section>

      <Modal
        title={`${activeQr?.label || ''}二维码`}
        visible={Boolean(activeQr)}
        footer={null}
        onCancel={() => setActiveQr(null)}
        centered
      >
        <div className='text-sm text-muted-foreground mb-4'>
          扫码添加微信联系{activeQr?.label}
        </div>
        <div className='flex justify-center p-4'>
          {activeQr && (
            <img
              src={activeQr.qr}
              alt={`${activeQr.label} 二维码`}
              className='w-72 h-72 object-contain rounded-lg bg-white p-3 opacity-80 border border-border'
            />
          )}
        </div>
      </Modal>
    </main>
  );
};

const About = () => {
  const { t } = useTranslation();
  const [about, setAbout] = useState('');
  const [aboutLoaded, setAboutLoaded] = useState(false);

  const displayAbout = async () => {
    const cachedAbout = localStorage.getItem('about') || '';
    setAbout(cachedAbout);

    try {
      const res = await API.get('/api/about');
      const { success, message, data } = res.data;
      if (success) {
        if (!data) {
          setAbout('');
          localStorage.removeItem('about');
          return;
        }

        let aboutContent = data;
        if (!data.startsWith('https://')) {
          aboutContent = marked.parse(data);
        }
        setAbout(aboutContent);
        localStorage.setItem('about', aboutContent);
      } else {
        showError(message);
        setAbout(t('加载关于内容失败...'));
      }
    } catch (error) {
      showError(t('加载关于内容失败...'));
      setAbout(cachedAbout);
    } finally {
      setAboutLoaded(true);
    }
  };

  useEffect(() => {
    displayAbout().then();
  }, []);

  return (
    <div className='mt-[60px]'>
      {aboutLoaded && about === '' ? (
        <AboutUsContent />
      ) : (
        <>
          {about.startsWith('https://') ? (
            <iframe
              src={about}
              style={{ width: '100%', height: '100vh', border: 'none' }}
            />
          ) : (
            <div
              style={{ fontSize: 'larger' }}
              dangerouslySetInnerHTML={{ __html: about }}
            ></div>
          )}
        </>
      )}
    </div>
  );
};

export default About;
