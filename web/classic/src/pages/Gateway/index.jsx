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

import React, {
  useContext,
  useEffect,
  useLayoutEffect,
  useMemo,
  useState,
} from 'react';
import {
  Activity,
  ArrowRight,
  Boxes,
  CheckCircle2,
  Code2,
  Copy,
  Globe2,
  Layers,
  Network,
  Plug,
  ServerCog,
  Sparkles,
  TrendingDown,
  Wallet,
  Workflow,
  Zap,
} from 'lucide-react';
import { Link } from 'react-router-dom';
import { API, copy, showError, showSuccess } from '../../helpers';
import { StatusContext } from '../../context/Status';

const apiDocsUrl = 'https://xmodel.apifox.cn';

const painPoints = [
  {
    icon: Globe2,
    title: '官方 API 接入复杂',
    desc: '不同供应商账号、计费与网络环境门槛高,直接接入成本大、周期长。',
    color: 'blue',
  },
  {
    icon: Layers,
    title: '不同模型接口不统一',
    desc: '各厂商 API 格式各异,多模型切换需要重写逻辑。统一入口可大幅降低集成成本。',
    color: 'purple',
  },
  {
    icon: TrendingDown,
    title: '调用成本高',
    desc: '直连官方按美元计费、无批量优惠,难以做成本控制。需要 AI API 代理与统一计费能力。',
    color: 'cyan',
  },
  {
    icon: Activity,
    title: '国内访问不稳定',
    desc: '直连海外服务延迟高、易超时。通过 API 中转与国内节点,实现稳定、高速的调用。',
    color: 'emerald',
  },
];

const solutions = [
  {
    icon: Plug,
    title: '统一 API 接口',
    desc: '已配置模型兼容 OpenAI Chat Completions 格式。无需修改业务代码,仅需更改 Base URL。',
    color: 'blue',
  },
  {
    icon: Workflow,
    title: '多模型智能路由',
    desc: '支持负载均衡与 Fallback。主模型限流或不可用时自动切换备用模型,保障高可用调用。',
    color: 'purple',
  },
  {
    icon: Wallet,
    title: '统一计费系统',
    desc: '人民币计费、按量付费。按项目与密钥细分账单,让平台成本透明可控。',
    color: 'cyan',
  },
  {
    icon: ServerCog,
    title: '高可用架构',
    desc: '多地域节点、自动容灾。AI API 代理层保证稳定高速,满足生产级并发与 SLA 要求。',
    color: 'emerald',
  },
];

const advantages = [
  {
    icon: TrendingDown,
    title: '低成本调用',
    desc: '人民币计费、透明定价,多模型比价。统一采购降低调用成本。',
    color: 'blue',
  },
  {
    icon: Zap,
    title: '稳定高并发',
    desc: '多地域节点、智能调度,支持企业级 QPS,满足 SaaS 与产品化需求。',
    color: 'purple',
  },
  {
    icon: Boxes,
    title: '多模型聚合',
    desc: '一个 API Key 调用平台已开通的模型账号池。',
    color: 'cyan',
  },
  {
    icon: Code2,
    title: '开发者友好',
    desc: '完整文档、SDK 示例、状态页与工单支持,让你专注业务创新。',
    color: 'emerald',
  },
];

const steps = [
  {
    n: '01',
    title: '注册账号',
    desc: '30 秒完成注册,无需海外信用卡。',
    color: 'blue-cyan',
  },
  {
    n: '02',
    title: '获取 API Key',
    desc: '创建密钥,充值后即可调用已开通模型。',
    color: 'purple-blue',
  },
  {
    n: '03',
    title: '调用 API',
    desc: '替换 Base URL,沿用现有 OpenAI 代码。',
    color: 'cyan-emerald',
  },
  {
    n: '04',
    title: '构建 AI 应用',
    desc: '接入生产环境,为用户提供智能服务。',
    color: 'amber-purple',
  },
];

const defaultModels = [
  'DeepSeek R1',
  'DeepSeek V3.2',
  '通义千问 Qwen',
  'Qwen Coder',
  'Kimi K2',
  '智谱 GLM',
  'MiniMax',
  '豆包 Doubao',
  '文心 ERNIE',
  '腾讯混元',
  '百川 Baichuan',
  '讯飞星火',
  '阶跃星辰 Step',
  '零一万物 Yi',
  '通义万相 Wan',
];

const modelPalette = [
  'from-brand-emerald to-brand-cyan',
  'from-brand-amber to-brand-purple',
  'from-brand-blue to-brand-cyan',
  'from-brand-purple to-brand-blue',
  'from-brand-cyan to-brand-blue',
  'from-brand-cyan to-brand-emerald',
];

const buildSampleCurl = (baseUrl) => `curl ${baseUrl}/v1/chat/completions \\
  -H "Content-Type: application/json" \\
  -H "Authorization: Bearer $XMODE_API_KEY" \\
  -d '{
    "model": "glm-4.5",
    "messages": [
      { "role": "user", "content": "Hello" }
    ]
  }'`;

const getFallbackGatewayBaseUrl = () =>
  (
    import.meta.env.VITE_REACT_APP_SERVER_URL ||
    window.location.origin
  ).replace(/\/$/, '');

const toGatewayModel = (name, index) => ({
  name,
  tag: '模型服务',
  color: modelPalette[index % modelPalette.length],
});

const cn = (...classes) => classes.filter(Boolean).join(' ');

const Card = ({ className = '', children }) => (
  <div
    className={cn(
      'rounded-lg border bg-card text-card-foreground shadow-sm',
      className,
    )}
  >
    {children}
  </div>
);

const GatewayButton = ({
  as: Component = 'button',
  size = 'default',
  variant = 'default',
  className = '',
  children,
  ...props
}) => {
  const variants = {
    default: 'bg-primary text-primary-foreground hover:bg-primary/90',
    outline:
      'border border-input bg-background hover:bg-accent hover:text-accent-foreground',
    ghost: 'hover:bg-accent hover:text-accent-foreground',
  };
  const sizes = {
    default: 'h-10 px-4 py-2',
    sm: 'h-9 rounded-md px-3',
    lg: 'h-11 rounded-md px-8',
  };

  return (
    <Component
      className={cn(
        'inline-flex items-center justify-center gap-2 whitespace-nowrap rounded-md text-sm font-medium transition-colors focus-visible:outline-none disabled:pointer-events-none disabled:opacity-50 [&_svg]:pointer-events-none [&_svg]:size-4 [&_svg]:shrink-0',
        variants[variant],
        sizes[size],
        className,
      )}
      {...props}
    >
      {children}
    </Component>
  );
};

const ModelCard = ({ model }) => (
  <div className='group flex min-w-[260px] items-center gap-3 rounded-2xl border border-white/10 bg-card/70 px-5 py-3 backdrop-blur-md hover-lift hover:border-brand-blue/50 hover:shadow-[0_0_24px_-6px_hsl(var(--brand-blue)/0.5)]'>
    <span
      className={`grid h-9 w-9 place-items-center rounded-xl bg-gradient-to-br ${model.color} shadow-[0_0_18px_-4px_hsl(var(--brand-purple)/0.5)]`}
    >
      <Sparkles className='h-4 w-4 text-white' />
    </span>
    <div className='flex flex-col'>
      <span className='whitespace-nowrap text-sm font-medium leading-tight text-foreground'>
        {model.name}
      </span>
      <span className='whitespace-nowrap text-xs text-muted-foreground'>
        {model.tag}
      </span>
    </div>
  </div>
);

const ModelRow = ({ items, animation, offset = '' }) => {
  const loop = [...items, ...items, ...items];

  return (
    <div className={`flex w-max gap-4 ${animation} ${offset}`}>
      {loop.map((model, index) => (
        <ModelCard key={`${model.name}-${index}`} model={model} />
      ))}
    </div>
  );
};

const Gateway = () => {
  const [statusState] = useContext(StatusContext);
  const [gatewayBaseUrl, setGatewayBaseUrl] = useState(() =>
    getFallbackGatewayBaseUrl(),
  );

  useLayoutEffect(() => {
    if (window.location.hash !== '#platform-capabilities') return;
    const target = document.getElementById('platform-capabilities');
    if (!target) return;
    window.requestAnimationFrame(() => {
      const top = target.getBoundingClientRect().top + window.scrollY - 80;
      window.scrollTo({ top, behavior: 'auto' });
    });
  }, []);

  useEffect(() => {
    const publicBaseUrl = statusState?.status?.public_api_base_url;
    if (publicBaseUrl) {
      setGatewayBaseUrl(publicBaseUrl.replace(/\/$/, ''));
      return;
    }

    API.get('/api/status')
      .then((res) => {
        const nextBaseUrl = res.data?.data?.public_api_base_url;
        if (nextBaseUrl) {
          setGatewayBaseUrl(nextBaseUrl.replace(/\/$/, ''));
        }
      })
      .catch(() => {});
  }, [statusState?.status?.public_api_base_url]);

  const docsLink = statusState?.status?.docs_link || apiDocsUrl;
  const sampleCurl = useMemo(
    () => buildSampleCurl(gatewayBaseUrl),
    [gatewayBaseUrl],
  );
  const marqueeModels = defaultModels.slice(0, 15).map(toGatewayModel);
  const modelRows = [
    marqueeModels.slice(0, 5),
    marqueeModels.slice(5, 10),
    marqueeModels.slice(10, 15),
  ];

  const handleCopyCode = async () => {
    const okay = await copy(sampleCurl);
    if (okay) {
      showSuccess('调用示例已复制');
      return;
    }
    showError('复制失败,请手动复制');
  };

  return (
    <main className='min-h-screen bg-background text-foreground'>
      <section className='pt-32 pb-16 relative overflow-hidden'>
        <div className='absolute inset-0 bg-hero-radial opacity-70' />
        <div className='absolute top-20 left-1/2 -translate-x-1/2 w-[700px] h-[400px] rounded-full bg-brand-blue/10 blur-[140px] pointer-events-none' />
        <div className='relative container mx-auto px-6'>
          <div className='grid lg:grid-cols-2 gap-10 items-center'>
            <div>
              <div className='inline-flex items-center gap-2 px-3 py-1 rounded-full border border-white/10 bg-card/60 backdrop-blur text-xs text-muted-foreground mb-6'>
                <Network className='w-3.5 h-3.5 text-brand-blue' />
                <span>API Gateway · 中转平台</span>
              </div>
              <h1 className='text-4xl md:text-6xl font-semibold text-architectural mb-5'>
                <span className='text-foreground'>API</span> 中转平台
              </h1>
              <p className='text-muted-foreground text-lg mb-3'>
                通过一个 API 调用平台已开通的 AI 模型。
              </p>
              <p className='text-muted-foreground mb-8'>
                完全兼容 OpenAI API 格式,几分钟即可完成接入。
              </p>
              <div className='flex flex-wrap items-center gap-3'>
                <Link to='/console/token'>
                  <GatewayButton className='rounded-full bg-gradient-to-r from-brand-blue to-brand-purple text-white hover:opacity-90 border-0 shadow-[0_0_24px_-4px_hsl(var(--brand-blue)/0.6)]'>
                    获取 API Key <ArrowRight className='w-4 h-4 ml-1' />
                  </GatewayButton>
                </Link>
                <a href={docsLink} target='_blank' rel='noopener noreferrer'>
                  <GatewayButton
                    variant='outline'
                    className='rounded-full border-white/10'
                  >
                    查看 API 文档
                  </GatewayButton>
                </a>
              </div>
            </div>

            <Card className='overflow-hidden border-white/10 bg-card/60 backdrop-blur shadow-[0_0_60px_-20px_hsl(var(--brand-purple)/0.4)]'>
              <div className='flex items-center justify-between px-4 py-2.5 border-b border-white/10 bg-gradient-to-r from-brand-blue/5 to-brand-purple/5'>
                <div className='flex items-center gap-1.5'>
                  <span className='w-2.5 h-2.5 rounded-full bg-red-400/70' />
                  <span className='w-2.5 h-2.5 rounded-full bg-yellow-400/70' />
                  <span className='w-2.5 h-2.5 rounded-full bg-green-400/70' />
                </div>
                <span className='text-xs uppercase tracking-widest text-muted-foreground'>
                  cURL · /v1/chat/completions
                </span>
                <GatewayButton
                  size='sm'
                  variant='ghost'
                  className='h-6 text-xs'
                  onClick={handleCopyCode}
                >
                  <Copy className='w-3 h-3 mr-1' /> 复制
                </GatewayButton>
              </div>
              <pre className='p-5 text-xs text-foreground/90 font-mono leading-relaxed overflow-x-auto'>
                <code>{sampleCurl}</code>
              </pre>
            </Card>
          </div>
        </div>
      </section>

      <section id='platform-capabilities' className='scroll-mt-24 py-16'>
        <div className='container mx-auto px-6'>
          <div className='text-center mb-10'>
            <div className='text-xs uppercase tracking-widest text-brand-purple mb-2'>
              Pain Points
            </div>
            <h2 className='text-3xl md:text-4xl font-semibold text-foreground'>
              为什么需要 AI API 中转
            </h2>
            <p className='text-muted-foreground mt-3'>
              国内 AI 开发者与产品团队常面临的接入难题,我们一站式解决。
            </p>
          </div>

          <div className='grid sm:grid-cols-2 lg:grid-cols-4 gap-4'>
            {painPoints.map((item) => {
              const Icon = item.icon;
              return (
                <Card
                  key={item.title}
                  className='p-6 bg-card/60 backdrop-blur border-white/10 hover:border-brand-purple/50 hover:shadow-[0_0_30px_-12px_hsl(var(--brand-purple)/0.5)] transition-all'
                >
                  <span className='grid place-items-center w-10 h-10 rounded-lg bg-brand-purple/10 text-brand-purple mb-4'>
                    <Icon className='w-5 h-5' />
                  </span>
                  <div className='text-foreground font-medium mb-1.5'>
                    {item.title}
                  </div>
                  <p className='text-sm text-muted-foreground leading-relaxed'>
                    {item.desc}
                  </p>
                </Card>
              );
            })}
          </div>
        </div>
      </section>

      <section className='py-16'>
        <div className='container mx-auto px-6'>
          <div className='text-center mb-10'>
            <div className='text-xs uppercase tracking-widest text-brand-blue mb-2'>
              Our Solution
            </div>
            <h2 className='text-3xl md:text-4xl font-semibold text-foreground'>
              我们的解决方案
            </h2>
            <p className='text-muted-foreground mt-3'>
              企业级 AI API 中转与网关能力,为国内 AI 开发者与 SaaS 团队打造。
            </p>
          </div>

          <div className='grid gap-4 sm:grid-cols-2'>
            {solutions.map((item) => {
              const Icon = item.icon;
              return (
                <Card
                  key={item.title}
                  className='group p-6 bg-card/60 backdrop-blur border-white/10 hover:border-brand-blue/50 hover:shadow-[0_0_30px_-12px_hsl(var(--brand-blue)/0.5)] transition-all'
                >
                  <div className='flex items-start gap-4'>
                    <span className='grid place-items-center w-11 h-11 rounded-xl bg-gradient-to-br from-brand-blue/20 to-brand-purple/20 text-brand-blue group-hover:from-brand-blue group-hover:to-brand-purple group-hover:text-white transition-all shrink-0'>
                      <Icon className='w-5 h-5' />
                    </span>
                    <div>
                      <h3 className='m-0 mb-1.5 text-lg font-medium text-foreground'>
                        {item.title}
                      </h3>
                      <p className='m-0 text-sm leading-relaxed text-muted-foreground'>
                        {item.desc}
                      </p>
                    </div>
                  </div>
                </Card>
              );
            })}
          </div>
        </div>
      </section>

      <section className='py-16 overflow-hidden border-y border-white/10 bg-card/30'>
        <div className='container mx-auto px-6 mb-10'>
          <div className='text-center mb-10'>
            <div className='text-xs uppercase tracking-widest text-brand-purple mb-2'>
              Supported Models
            </div>
            <h2 className='text-3xl md:text-4xl font-semibold text-foreground'>
              支持模型
            </h2>
          </div>
        </div>
        <div className='relative space-y-4'>
          <ModelRow items={modelRows[0]} animation='animate-marquee' />
          {modelRows[1].length > 0 && (
            <ModelRow
              items={modelRows[1]}
              animation='animate-marquee-reverse'
              offset='ml-16'
            />
          )}
          {modelRows[2].length > 0 && (
            <ModelRow
              items={modelRows[2]}
              animation='animate-marquee-slow'
              offset='ml-32'
            />
          )}
          <div className='absolute inset-y-0 left-0 w-40 bg-gradient-to-r from-background to-transparent pointer-events-none' />
          <div className='absolute inset-y-0 right-0 w-40 bg-gradient-to-l from-background to-transparent pointer-events-none' />
        </div>
      </section>

      <section className='py-16'>
        <div className='container mx-auto px-6'>
          <Card className='overflow-hidden border-white/10 bg-card/60 backdrop-blur'>
            <div className='grid gap-0 md:grid-cols-2'>
              <div className='p-8 md:p-10'>
                <div className='text-xs uppercase tracking-widest text-brand-blue mb-3'>
                  OpenAI Compatible
                </div>
                <h2 className='text-2xl md:text-3xl font-semibold text-foreground mb-4'>
                  兼容 OpenAI API 接口
                </h2>
                <p className='text-muted-foreground leading-relaxed mb-6'>
                  无需修改代码,直接使用 OpenAI SDK,仅将 Base URL 指向 Xmodel AI
                  API 平台即可。
                </p>
                <ul className='space-y-3 text-sm'>
                  {[
                    '无需修改业务代码',
                    '兼容 OpenAI SDK 与 cURL',
                    '支持流式输出与 Function Calling',
                  ].map((text) => (
                    <li
                      key={text}
                      className='flex items-center gap-2 text-foreground'
                    >
                      <CheckCircle2 className='w-4 h-4 text-brand-blue' />
                      {text}
                    </li>
                  ))}
                </ul>
              </div>
              <div className='border-l border-white/10 bg-gradient-to-br from-brand-blue/5 to-brand-purple/5 p-6'>
                <div className='flex items-center justify-between mb-3'>
                  <span className='text-xs text-muted-foreground'>
                    curl · POST /v1/chat/completions
                  </span>
                  <button
                    type='button'
                    className='inline-flex items-center gap-1 text-xs text-muted-foreground transition-colors hover:text-foreground'
                    onClick={handleCopyCode}
                  >
                    <Copy size={14} /> 复制
                  </button>
                </div>
                <pre className='text-xs text-foreground/90 font-mono leading-relaxed overflow-x-auto'>
                  <code>{sampleCurl}</code>
                </pre>
              </div>
            </div>
          </Card>
        </div>
      </section>

      <section className='py-16'>
        <div className='container mx-auto px-6'>
          <div className='text-center mb-10'>
            <div className='text-xs uppercase tracking-widest text-brand-blue mb-2'>
              Quickstart
            </div>
            <h2 className='text-3xl md:text-4xl font-semibold text-foreground'>
              四步接入,快速开始
            </h2>
            <p className='text-muted-foreground mt-3'>
              注册即可获取 API Key,立即体验 OpenAI API 中转与多模型聚合。
            </p>
          </div>

          <div className='grid sm:grid-cols-2 lg:grid-cols-4 gap-4'>
            {steps.map((step) => (
              <Card
                key={step.n}
                className='p-6 bg-card/60 backdrop-blur border-white/10 hover:border-brand-blue/50 hover:shadow-[0_0_30px_-12px_hsl(var(--brand-blue)/0.5)] transition-all'
              >
                <div className='text-3xl font-semibold text-gradient-brand mb-3'>
                  {step.n}
                </div>
                <div className='text-foreground font-medium mb-1.5'>
                  {step.title}
                </div>
                <p className='text-sm text-muted-foreground leading-relaxed'>
                  {step.desc}
                </p>
              </Card>
            ))}
          </div>
        </div>
      </section>

      <section className='py-16'>
        <div className='container mx-auto px-6'>
          <div className='text-center mb-10'>
            <div className='text-xs uppercase tracking-widest text-brand-purple mb-2'>
              Why Xmodel
            </div>
            <h2 className='text-3xl md:text-4xl font-semibold text-foreground'>
              平台优势
            </h2>
            <p className='text-muted-foreground mt-3'>
              为国内 AI 应用开发者与 SaaS 团队打造的 AI API 平台,稳定、省心、易用。
            </p>
          </div>

          <div className='grid sm:grid-cols-2 lg:grid-cols-4 gap-4'>
            {advantages.map((item) => {
              const Icon = item.icon;
              return (
                <Card
                  key={item.title}
                  className='group p-6 bg-card/60 backdrop-blur border-white/10 hover:border-brand-blue/50 hover:shadow-[0_0_30px_-12px_hsl(var(--brand-blue)/0.5)] transition-all'
                >
                  <span className='grid place-items-center w-11 h-11 rounded-xl bg-gradient-to-br from-brand-blue/20 to-brand-purple/20 text-brand-blue group-hover:from-brand-blue group-hover:to-brand-purple group-hover:text-white transition-all mb-4'>
                    <Icon className='w-5 h-5' />
                  </span>
                  <div className='text-foreground font-medium mb-1.5'>
                    {item.title}
                  </div>
                  <p className='text-sm text-muted-foreground leading-relaxed'>
                    {item.desc}
                  </p>
                </Card>
              );
            })}
          </div>
        </div>
      </section>

      <section className='py-20 pb-24 relative overflow-hidden'>
        <div className='absolute top-0 left-1/2 -translate-x-1/2 w-[600px] h-[400px] rounded-full bg-brand-blue/10 blur-[140px] pointer-events-none' />
        <div className='relative container mx-auto px-6 text-center'>
          <Sparkles className='w-7 h-7 text-brand-blue mx-auto mb-4' />
          <h2 className='text-3xl md:text-5xl font-semibold text-architectural mb-5'>
            立即开始使用 <span className='text-gradient-brand'>AI API</span>
          </h2>
          <p className='text-muted-foreground mb-8'>
            注册即获 API Key,稳定调用平台已开通模型。OpenAI API 中转,一键接入。
          </p>
          <Link to='/console/token'>
            <GatewayButton
              size='lg'
              className='rounded-full h-12 px-8 bg-gradient-to-r from-brand-blue to-brand-purple text-white hover:opacity-90 border-0 shadow-[0_0_40px_-8px_hsl(var(--brand-blue)/0.7)]'
            >
              获取 API Key <ArrowRight className='w-4 h-4 ml-1' />
            </GatewayButton>
          </Link>
        </div>
      </section>
    </main>
  );
};

export default Gateway;
