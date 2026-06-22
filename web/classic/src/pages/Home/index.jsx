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

import React, { useContext, useEffect, useMemo, useRef, useState } from 'react';
import { Link } from 'react-router-dom';
import { marked } from 'marked';
import {
  ArrowRight,
  BookOpen,
  Bot,
  Cable,
  Check,
  Code2,
  Copy as CopyIcon,
  Cpu,
  Globe2,
  PenLine,
  Search,
  Shuffle,
  Sparkles,
  Terminal,
  Wallet,
  Wrench,
  Zap,
} from 'lucide-react';
import { API, copy, showError, showSuccess } from '../../helpers';
import { useIsMobile } from '../../hooks/common/useIsMobile';
import { StatusContext } from '../../context/Status';
import { useActualTheme } from '../../context/Theme';
import { useTranslation } from 'react-i18next';
import NoticeModal from '../../components/layout/NoticeModal';

const LINE_COUNT = 36;
const ROW_COUNT = 12;
const ROTATION_DURATION = 12000;
const LINE_ROTATION_STEP = Math.PI / 24;
const GRID_HEIGHT_RATIO = 0.52;
const MAX_PIXEL_RATIO = 1.5;
const MAX_DESKTOP_PIXEL_RATIO = 1;
const DESKTOP_CANVAS_AREA = 900000;
const SPRITE_SIZE = 96;
const DIGIT_COLOR = '#c084fc';
const GLOW_COLOR = '#a855f7';

const POINTS = Array.from({ length: LINE_COUNT * ROW_COUNT }, (_, index) => ({
  digit: Math.floor(Math.random() * 10),
  lineIndex: index % LINE_COUNT,
  rowIndex: Math.floor(index / LINE_COUNT),
}));

const modelPalette = [
  'from-brand-emerald to-brand-cyan',
  'from-brand-amber to-brand-purple',
  'from-brand-blue to-brand-cyan',
  'from-brand-purple to-brand-blue',
  'from-brand-cyan to-brand-blue',
  'from-brand-cyan to-brand-emerald',
];

const chinaFallbackModels = [
  { name: 'DeepSeek R1', tag: '推理 / 通用助手', color: modelPalette[0] },
  { name: 'DeepSeek V3.2', tag: '通用助手 / 代码', color: modelPalette[1] },
  { name: '通义千问 Qwen', tag: '通用助手 / 长文本', color: modelPalette[2] },
  { name: 'Qwen Coder', tag: '代码生成 / Agent', color: modelPalette[3] },
  { name: 'Kimi K2', tag: '长上下文 / 推理', color: modelPalette[4] },
  { name: '智谱 GLM', tag: '通用助手 / 多模态', color: modelPalette[5] },
  { name: 'MiniMax', tag: '通用助手 / 语音', color: modelPalette[0] },
  { name: '豆包 Doubao', tag: '通用助手 / 内容创作', color: modelPalette[1] },
  { name: '文心 ERNIE', tag: '知识问答 / 创作', color: modelPalette[2] },
  { name: '腾讯混元', tag: '通用助手 / 企业应用', color: modelPalette[3] },
  { name: '百川 Baichuan', tag: '通用助手 / 领域知识', color: modelPalette[4] },
  { name: '讯飞星火', tag: '中文理解 / 办公', color: modelPalette[5] },
  { name: '阶跃星辰 Step', tag: '多模态 / 推理', color: modelPalette[0] },
  { name: '零一万物 Yi', tag: '通用助手 / 长文本', color: modelPalette[1] },
  { name: '通义万相 Wan', tag: '图像视频 / AIGC', color: modelPalette[2] },
];

const infrastructureItems = [
  {
    icon: Globe2,
    title: 'AI Gateway',
    desc: '全球部署的边缘加速节点,提供鉴权、限流、审计等完整 API 网关能力,确保您的每一次调用都毫秒级响应。',
    color: 'text-brand-blue',
    bg: 'bg-brand-blue/10',
    ring: 'ring-brand-blue/20',
  },
  {
    icon: Cable,
    title: '统一 API 接口',
    desc: '全系模型 100% 兼容 OpenAI 格式。无需重写现有业务逻辑,仅需更改一行 Base URL,即可接入顶尖模型。',
    color: 'text-brand-purple',
    bg: 'bg-brand-purple/10',
    ring: 'ring-brand-purple/20',
  },
  {
    icon: Shuffle,
    title: '多模型智能路由',
    desc: '支持模型负载均衡与 Fallback 机制。主模型限流或宕机时无缝切换至备用模型,保障业务高可用不停机。',
    color: 'text-brand-cyan',
    bg: 'bg-brand-cyan/10',
    ring: 'ring-brand-cyan/20',
  },
  {
    icon: Wallet,
    title: '成本优化',
    desc: '极其细粒度的花费全景看板视图。按项目、按模型、按密钥实时统计 Token 开销,让每一分预算都在掌控之中。',
    color: 'text-brand-emerald',
    bg: 'bg-brand-emerald/10',
    ring: 'ring-brand-emerald/20',
  },
];

const developerTabs = ['Python', 'Node.js', 'cURL'];

const developerFeatures = [
  {
    icon: Code2,
    title: '一行代码切换 URL',
    desc: '即可让现有应用接入已配置的模型。',
  },
  {
    icon: Zap,
    title: '支持流式输出 (SSE)',
    desc: '原生支持打字机效果响应,低延迟绝佳体验。',
  },
  {
    icon: Wrench,
    title: '支持工具调用 (Function Calling)',
    desc: '无缝使用主流模型的插件和工具调用能力。',
  },
];

const steps = [
  {
    n: '01',
    title: '注册账号',
    desc: '30 秒快速注册,无需绑定海外信用卡。',
    color: 'from-brand-blue to-brand-cyan',
  },
  {
    n: '02',
    title: '获取 API Key',
    desc: '生成专属密钥,充值即可使用全部模型。',
    color: 'from-brand-purple to-brand-blue',
  },
  {
    n: '03',
    title: '测试调用 API',
    desc: '复制提供的代码示例,在终端进行首次跑通验证。',
    color: 'from-brand-cyan to-brand-emerald',
  },
  {
    n: '04',
    title: '构建超级应用',
    desc: '将 API 集成到您的生产环境,为用户创造非凡价值。',
    color: 'from-brand-amber to-brand-purple',
  },
];

const scenarios = [
  {
    icon: Bot,
    title: '智能 AI 客服',
    desc: '利用大模型语境理解能力,全天候处理高频客诉,极大降低人工成本。',
    color: 'text-brand-blue',
    bg: 'bg-brand-blue/10',
  },
  {
    icon: PenLine,
    title: 'AI 写作助理',
    desc: '高并发流式生成优质内容。',
    color: 'text-brand-purple',
    bg: 'bg-brand-purple/10',
  },
  {
    icon: Cpu,
    title: 'AI Agent 系统',
    desc: '利用工具调用执行复杂任务。',
    color: 'text-brand-cyan',
    bg: 'bg-brand-cyan/10',
  },
  {
    icon: BookOpen,
    title: '企业知识库',
    desc: '搭配 RAG 实现专业私有问答。',
    color: 'text-brand-emerald',
    bg: 'bg-brand-emerald/10',
  },
  {
    icon: Search,
    title: '语义化 AI 搜索',
    desc: '打破关键字束缚,意图理解更精准匹配结果。',
    color: 'text-brand-amber',
    bg: 'bg-brand-amber/10',
  },
  {
    icon: Code2,
    title: 'AI 代码顾问',
    desc: '减少写代码工作量,提高代码的可读性和可维护性,让您更专注于业务逻辑。',
    color: 'text-brand-blue',
    bg: 'bg-brand-blue/10',
  },
];

const createCodeSnippets = (baseUrl) => ({
  Python: `import os
from openai import OpenAI

client = OpenAI(
    api_key=os.environ.get("APIMARKET_KEY"),
    base_url="${baseUrl}"
)

stream = client.chat.completions.create(
    model="glm-4.5",
    messages=[{"role": "user", "content": "你好,世界!"}],
    stream=True,
)

for chunk in stream:
    print(chunk.choices[0].delta.content or "", end="")`,
  'Node.js': `import { OpenAI } from 'openai';

const client = new OpenAI({
  apiKey: process.env.APIMARKET_KEY,
  baseURL: '${baseUrl}',
});

const stream = await client.chat.completions.create({
  model: 'glm-4.5',
  messages: [{ role: 'user', content: 'Hello' }],
  stream: true,
});

for await (const chunk of stream) {
  process.stdout.write(chunk.choices[0]?.delta?.content || '');
}`,
  cURL: `curl ${baseUrl}/chat/completions \\
  -H "Content-Type: application/json" \\
  -H "Authorization: Bearer $APIMARKET_KEY" \\
  -d '{
    "model": "glm-4.5",
    "messages": [{"role": "user", "content": "你能做什么?"}]
  }'`,
});

const getFallbackServerAddress = () =>
  (
    import.meta.env.VITE_REACT_APP_SERVER_URL ||
    window.location.origin
  ).replace(/\/$/, '');

const createDigitSprites = () =>
  Array.from({ length: 10 }, (_, digit) => {
    const sprite = document.createElement('canvas');
    const context = sprite.getContext('2d');
    sprite.width = SPRITE_SIZE;
    sprite.height = SPRITE_SIZE;

    if (!context) return sprite;

    const center = SPRITE_SIZE / 2;

    context.shadowBlur = 14;
    context.shadowColor = GLOW_COLOR;
    context.fillStyle = 'rgba(192, 132, 252, 0.16)';
    context.beginPath();
    context.arc(center, center, 12, 0, Math.PI * 2);
    context.fill();

    context.font = '16px ui-monospace, SFMono-Regular, Menlo, monospace';
    context.textAlign = 'center';
    context.textBaseline = 'middle';
    context.shadowBlur = 36;
    context.shadowColor = GLOW_COLOR;
    context.fillStyle = DIGIT_COLOR;
    context.fillText(String(digit), center, center);

    return sprite;
  });

const cn = (...classes) => classes.filter(Boolean).join(' ');

const HomeButton = ({
  as: Component = 'button',
  variant = 'default',
  size = 'default',
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

const MatrixRain = () => {
  const canvasRef = useRef(null);

  useEffect(() => {
    const canvas = canvasRef.current;
    if (!canvas) return;

    const context = canvas.getContext('2d', { alpha: true });
    if (!context) return;

    const motionQuery = window.matchMedia('(prefers-reduced-motion: reduce)');
    const digitSprites = createDigitSprites();
    let width = 0;
    let height = 0;
    let pixelRatio = 1;
    let animationFrame = 0;

    const resizeCanvas = () => {
      const bounds = canvas.getBoundingClientRect();
      const maxPixelRatio =
        bounds.width * bounds.height > DESKTOP_CANVAS_AREA
          ? MAX_DESKTOP_PIXEL_RATIO
          : MAX_PIXEL_RATIO;
      pixelRatio = Math.min(window.devicePixelRatio || 1, maxPixelRatio);
      width = Math.max(1, Math.round(bounds.width));
      height = Math.max(1, Math.round(bounds.height));
      canvas.width = Math.round(width * pixelRatio);
      canvas.height = Math.round(height * pixelRatio);
      context.setTransform(pixelRatio, 0, 0, pixelRatio, 0, 0);
      context.imageSmoothingEnabled = true;
    };

    const render = (time = 0) => {
      const reducedMotion = motionQuery.matches;
      const elapsed = reducedMotion ? 0 : time % ROTATION_DURATION;
      const rotation = (elapsed / ROTATION_DURATION) * Math.PI * 2;
      const gridWidth = width * 0.9;
      const gridHeight = height * GRID_HEIGHT_RATIO;
      const centerX = width / 2;
      const centerY = height * 0.46;
      const perspective = Math.max(width, height) * 0.76;
      const digitBoxHeight = gridHeight / 18;
      const rowGap = (gridHeight - digitBoxHeight) / Math.max(1, ROW_COUNT - 1);
      const lineGap = gridWidth / Math.max(1, LINE_COUNT - 1);
      const halfGridHeight = gridHeight / 2;

      context.clearRect(0, 0, width, height);

      for (const point of POINTS) {
        const localX = -gridWidth / 2 + point.lineIndex * lineGap;
        const localY =
          -gridHeight / 2 + digitBoxHeight / 2 + point.rowIndex * rowGap;
        const angle = point.lineIndex * LINE_ROTATION_STEP + rotation;
        const rotatedY = localY * Math.cos(angle);
        const depth = localY * Math.sin(angle);
        const perspectiveScale = perspective / (perspective - depth);
        const scale = Math.min(1.55, Math.max(0.58, perspectiveScale));
        const depthRatio = Math.min(
          1,
          Math.max(0, depth / halfGridHeight / 2 + 0.5),
        );
        const alpha = 0.34 + depthRatio * 0.42;
        const sprite = digitSprites[point.digit];
        const spriteSize = SPRITE_SIZE * scale;
        const x = centerX + localX * perspectiveScale;
        const y = centerY + rotatedY * perspectiveScale;

        context.globalAlpha = alpha;
        context.drawImage(
          sprite,
          x - spriteSize / 2,
          y - spriteSize / 2,
          spriteSize,
          spriteSize,
        );
      }

      context.globalAlpha = 1;
    };

    const tick = (time) => {
      render(time);
      animationFrame = window.requestAnimationFrame(tick);
    };

    resizeCanvas();
    render();
    if (!motionQuery.matches) {
      animationFrame = window.requestAnimationFrame(tick);
    }

    const resizeObserver = new ResizeObserver(() => {
      resizeCanvas();
      render();
    });
    resizeObserver.observe(canvas);

    return () => {
      window.cancelAnimationFrame(animationFrame);
      resizeObserver.disconnect();
    };
  }, []);

  return (
    <div className='matrix-rain pointer-events-none' aria-hidden='true'>
      <canvas ref={canvasRef} className='block h-full w-full' />
    </div>
  );
};

const parseModelTag = (row) => {
  try {
    const tags = JSON.parse(row.tags || row.scenarios_json || '[]');
    if (Array.isArray(tags) && tags.length > 0) return tags.slice(0, 2).join(' / ');
  } catch {
    if (typeof row.tags === 'string' && row.tags.trim()) {
      return row.tags.split(/[,;|\s]+/).filter(Boolean).slice(0, 2).join(' / ');
    }
  }
  return row.type || '模型服务';
};

const toMarqueeModel = (row, index) => ({
  name:
    row.vendor_name && row.model_name
      ? `${row.vendor_name} ${row.model_name}`
      : row.name || row.model_name || row.slug || 'Model',
  tag: parseModelTag(row),
  color: modelPalette[index % modelPalette.length],
});

const ModelCard = ({ model }) => (
  <div className='group flex items-center gap-3 rounded-2xl border border-white/10 bg-card/70 backdrop-blur-md px-5 py-3 min-w-[260px] hover:border-brand-blue/50 hover:shadow-[0_0_24px_-6px_hsl(var(--brand-blue)/0.5)] hover-lift'>
    <span
      className={`grid place-items-center w-9 h-9 rounded-xl bg-gradient-to-br ${model.color} shadow-[0_0_18px_-4px_hsl(var(--brand-purple)/0.5)]`}
    >
      <Sparkles className='w-4 h-4 text-white' />
    </span>
    <div className='flex flex-col'>
      <span className='text-sm font-medium text-foreground whitespace-nowrap leading-tight'>
        {model.name}
      </span>
      <span className='text-xs text-muted-foreground whitespace-nowrap'>
        {model.tag}
      </span>
    </div>
  </div>
);

const ModelRow = ({ items, animation, offset = '' }) => {
  const loop = [...items, ...items, ...items];
  return (
    <div className={`flex gap-4 w-max ${animation} ${offset}`}>
      {loop.map((model, index) => (
        <ModelCard key={`${model.name}-${index}`} model={model} />
      ))}
    </div>
  );
};

const Hero = ({ endpoint, docsLink, onCopyEndpoint }) => (
  <section className='relative min-h-screen flex items-center justify-center overflow-hidden pt-10 bg-white dark:bg-transparent'>
    <div className='absolute inset-0 bg-grid opacity-60' />
    <div className='absolute inset-0 bg-hero-radial' />
    <MatrixRain />
    <div className='absolute top-1/3 left-1/4 w-[400px] h-[400px] rounded-full bg-brand-blue/20 blur-[120px] pointer-events-none' />
    <div className='absolute bottom-1/4 right-1/4 w-[400px] h-[400px] rounded-full bg-brand-purple/20 blur-[120px] pointer-events-none' />

    <div className='relative z-10 container mx-auto px-6 text-center'>
      <button
        type='button'
        onClick={onCopyEndpoint}
        disabled={!endpoint}
        className='xmodel-home-endpoint inline-flex max-w-full items-center gap-2 rounded-full border border-white/10 bg-card/60 backdrop-blur-md px-4 py-1.5 mb-10 reveal shadow-[0_2px_20px_rgba(0,0,0,0.6)] transition hover:border-brand-blue/40 hover:text-foreground'
        title='复制 endpoint'
      >
        <CopyIcon className='h-3.5 w-3.5 shrink-0 text-brand-blue' />
        <span className='truncate text-xs font-mono text-muted-foreground [text-shadow:_0_2px_8px_rgba(0,0,0,0.8)]'>
          {endpoint || 'Endpoint 加载中'}
        </span>
      </button>

      <h1 className='xmodel-home-title text-[3rem] md:text-[4.2rem] lg:text-[5rem] font-semibold text-architectural mb-8 reveal leading-[1.1] [text-shadow:_0_4px_24px_rgba(0,0,0,0.7)] dark:[text-shadow:none]'>
        <span className='block text-foreground'>AI主流模型</span>
        <span className='xmodel-home-title-gradient block mt-2 bg-gradient-to-r from-brand-blue to-brand-purple bg-clip-text text-transparent drop-shadow-[0_0_24px_hsl(var(--brand-blue)/0.6)] dark:drop-shadow-none'>
          API 聚合平台
        </span>
      </h1>

      <p className='xmodel-home-desc text-base md:text-lg font-bold text-muted-foreground max-w-2xl mx-auto mb-10 reveal-delayed leading-relaxed [text-shadow:_0_2px_12px_rgba(0,0,0,0.85)] dark:[text-shadow:none]'>
        聚合平台提供各大 AI 模型接口中转聚合管理服务，仅用一个接口即可体验各大 AI
        公司的三百多种大模型。全面解决 AI 访问难题，为您的业务赋能。
      </p>

      <div className='flex flex-wrap items-center justify-center gap-4 reveal-delayed'>
        <HomeButton
          as={Link}
          to='/console/token'
          size='lg'
          className='rounded-full h-12 px-7 bg-gradient-to-r from-brand-blue to-brand-purple text-white hover:opacity-90 border-0 shadow-[0_0_40px_-8px_hsl(var(--brand-blue)/0.7)]'
        >
          <Zap className='w-4 h-4 mr-1' />
          获取 API Key
          <ArrowRight className='w-4 h-4 ml-1' />
        </HomeButton>
        <HomeButton
          as='a'
          href={docsLink}
          target={docsLink === '#' ? undefined : '_blank'}
          rel={docsLink === '#' ? undefined : 'noopener noreferrer'}
          onClick={
            docsLink === '#' ? (event) => event.preventDefault() : undefined
          }
          size='lg'
          variant='outline'
          className='rounded-full h-12 px-7 border-border/80 bg-card hover:bg-card/90'
        >
          <Terminal className='w-4 h-4 mr-2' />
          查看 API 文档
        </HomeButton>
      </div>

      <p className='mt-20 text-xs text-muted-foreground tracking-wider uppercase'>
        一点接入,驱动无限可能
      </p>
    </div>
  </section>
);

const ModelsMarquee = ({ items }) => {
  const [row1, row2, row3] = useMemo(() => {
    const visible = (items.length > 0 ? items : chinaFallbackModels).slice(
      0,
      15,
    );
    return [visible.slice(0, 5), visible.slice(5, 10), visible.slice(10, 15)];
  }, [items]);

  return (
    <section
      id='models'
      className='xmodel-home-section-band py-16 border-y overflow-hidden'
    >
      <div className='container mx-auto px-6 mb-10 text-center'>
        <p className='text-minimal text-brand-cyan mb-3'>MODEL HUB</p>
        <h2 className='text-4xl md:text-5xl font-semibold text-architectural'>
          一站接入<span className='text-gradient-brand'> 主流大模型</span>
        </h2>
      </div>
      <div className='relative space-y-4'>
        <ModelRow items={row1} animation='animate-marquee' />
        {row2.length > 0 && (
          <ModelRow
            items={row2}
            animation='animate-marquee-reverse'
            offset='ml-16'
          />
        )}
        {row3.length > 0 && (
          <ModelRow
            items={row3}
            animation='animate-marquee-slow'
            offset='ml-32'
          />
        )}
        <div className='absolute inset-y-0 left-0 w-40 bg-gradient-to-r from-background to-transparent pointer-events-none' />
        <div className='absolute inset-y-0 right-0 w-40 bg-gradient-to-l from-background to-transparent pointer-events-none' />
      </div>
    </section>
  );
};

const Infrastructure = () => (
  <section id='infrastructure' className='py-32 relative'>
    <div className='container mx-auto px-6'>
      <div className='max-w-3xl mx-auto mb-16 text-center'>
        <p className='text-minimal text-brand-blue mb-4'>INFRASTRUCTURE</p>
        <h2 className='text-3xl md:text-5xl font-semibold text-architectural mb-5'>
          为您打造的<span className='text-gradient-brand'>基础设施</span>
        </h2>
        <p className='text-muted-foreground text-lg'>
          不仅仅提供模型接口,我们提供支持海量并发的企业级 AI 中台架构。
        </p>
      </div>

      <div className='grid grid-cols-2 gap-4 md:gap-5'>
        {infrastructureItems.map((item) => {
          const Icon = item.icon;
          return (
            <div
              key={item.title}
              className='group relative glass-card rounded-2xl p-5 md:p-7 hover:border-border hover-lift'
            >
              <div
                className={`inline-flex items-center justify-center w-11 h-11 rounded-xl ${item.bg} ring-1 ${item.ring} mb-5`}
              >
                <Icon className={`w-5 h-5 ${item.color}`} />
              </div>
              <h3 className='text-xl font-semibold mb-2 text-foreground'>
                {item.title}
              </h3>
              <p className='text-muted-foreground leading-relaxed text-sm'>
                {item.desc}
              </p>
            </div>
          );
        })}
      </div>
    </div>
  </section>
);

const Developer = ({ snippets, activeTab, setActiveTab }) => (
  <section
    id='developer'
    className='xmodel-home-section-band overflow-hidden py-24 border-y md:py-32'
  >
    <div className='container mx-auto px-4 sm:px-6'>
      <div className='text-center mb-16 max-w-3xl mx-auto'>
        <p className='text-minimal text-brand-purple mb-4'>FOR DEVELOPERS</p>
        <h2 className='text-[34px] md:text-[52px] font-semibold text-architectural mb-5'>
          为<span className='text-gradient-brand'>开发者</span>而生
        </h2>
        <p className='text-muted-foreground text-lg leading-relaxed'>
          极简、优雅的接入体验。无论您使用哪种语言,这看起来都像极了您熟悉的
          OpenAI SDK。无缝接入,立马发版。
        </p>
      </div>
      <div className='grid min-w-0 gap-10 xl:grid-cols-[minmax(0,1fr)_minmax(0,1fr)] xl:items-center xl:gap-16'>
        <div className='min-w-0'>
          <p className='text-muted-foreground text-lg mb-10 leading-relaxed sr-only'>
            极简、优雅的接入体验。无论您使用哪种语言,这看起来都像极了您熟悉的
            OpenAI SDK。无缝接入,立马发版。
          </p>

          <div className='space-y-10'>
            {developerFeatures.map((feature) => (
              <div key={feature.title} className='flex gap-4'>
                <div className='shrink-0 mt-0.5 grid place-items-center w-8 h-8 rounded-lg bg-brand-emerald/10 ring-1 ring-brand-emerald/20'>
                  <Check className='w-4 h-4 text-brand-emerald' />
                </div>
                <div>
                  <h3 className='font-semibold text-foreground mb-1'>
                    {feature.title}
                  </h3>
                  <p className='text-sm text-muted-foreground'>
                    {feature.desc}
                  </p>
                </div>
              </div>
            ))}
          </div>
        </div>

        <div className='relative mx-auto w-full min-w-0 max-w-[640px] xl:max-w-full'>
          <div className='absolute -inset-2 rounded-3xl bg-gradient-to-br from-brand-blue/20 via-brand-purple/20 to-brand-cyan/20 blur-2xl sm:-inset-4' />
          <div className='relative max-w-full overflow-hidden rounded-2xl shadow-2xl glass-card'>
            <div className='xmodel-code-window-header flex min-w-0 items-center justify-between gap-3 border-b px-3 py-3 sm:px-4'>
              <div className='flex shrink-0 gap-1.5'>
                <span className='w-3 h-3 rounded-full bg-destructive/70' />
                <span className='w-3 h-3 rounded-full bg-brand-amber/70' />
                <span className='w-3 h-3 rounded-full bg-brand-emerald/70' />
              </div>
              <div className='flex min-w-0 flex-wrap justify-end gap-1'>
                {developerTabs.map((tab) => (
                  <button
                    key={tab}
                    type='button'
                    onClick={() => setActiveTab(tab)}
                    className={`rounded-md px-2 py-1 text-xs transition-colors sm:px-3 ${
                      activeTab === tab
                        ? 'bg-brand-blue/15 text-brand-blue'
                        : 'text-muted-foreground hover:text-foreground'
                    }`}
                  >
                    {tab}
                  </button>
                ))}
              </div>
            </div>
            <pre className='max-w-full overflow-x-auto p-4 font-mono text-[11px] leading-relaxed text-foreground/90 sm:p-5 sm:text-xs'>
              <code className='block min-w-max'>{snippets[activeTab]}</code>
            </pre>
          </div>
        </div>
      </div>
    </div>
  </section>
);

const Steps = () => (
  <section className='py-32'>
    <div className='container mx-auto px-6'>
      <div className='max-w-3xl mx-auto text-center mb-16'>
        <p className='text-minimal text-brand-cyan mb-4'>QUICK START</p>
        <h2 className='text-3xl md:text-5xl font-semibold text-architectural mb-5'>
          只需<span className='text-gradient-brand'>四步</span>,开启 AI 建设之路
        </h2>
      </div>

      <div className='grid grid-cols-2 lg:grid-cols-4 gap-4 md:gap-5 relative'>
        {steps.map((step, index) => (
          <div
            key={step.n}
            className='relative glass-card rounded-2xl p-5 md:p-7 group hover-lift'
          >
            <div
              className={`text-5xl font-bold bg-gradient-to-br ${step.color} bg-clip-text text-transparent mb-4`}
            >
              {step.n}
            </div>
            <h3 className='text-lg font-semibold text-foreground mb-2'>
              {step.title}
            </h3>
            <p className='text-sm text-muted-foreground leading-relaxed'>
              {step.desc}
            </p>
            {index < steps.length - 1 && (
              <div className='hidden lg:block absolute top-1/2 -right-3 w-6 h-px bg-gradient-to-r from-border to-transparent' />
            )}
          </div>
        ))}
      </div>
    </div>
  </section>
);

const Scenarios = () => (
  <section id='scenarios' className='xmodel-home-section-band py-32 border-y'>
    <div className='container mx-auto px-6'>
      <div className='max-w-3xl mx-auto mb-16 text-center'>
        <p className='text-minimal text-brand-emerald mb-4'>USE CASES</p>
        <h2 className='text-3xl md:text-5xl font-semibold text-architectural mb-5'>
          驱动<span className='text-gradient-brand'>全链路</span> AI 场景
        </h2>
        <p className='text-muted-foreground text-lg'>
          我们的基础架构可高度适配各类应用场景的并发与响应需求。
        </p>
      </div>

      <div className='grid grid-cols-2 gap-4 md:gap-5'>
        {scenarios.map((item) => {
          const Icon = item.icon;
          return (
            <div
              key={item.title}
              className='glass-card rounded-2xl p-5 md:p-7 hover-lift'
            >
              <div
                className={`inline-flex items-center justify-center w-11 h-11 rounded-xl ${item.bg} mb-5`}
              >
                <Icon className={`w-5 h-5 ${item.color}`} />
              </div>
              <h3 className='text-lg font-semibold text-foreground mb-2'>
                {item.title}
              </h3>
              <p className='text-sm text-muted-foreground leading-relaxed'>
                {item.desc}
              </p>
            </div>
          );
        })}
      </div>
    </div>
  </section>
);

const Home = () => {
  const { t, i18n } = useTranslation();
  const [statusState] = useContext(StatusContext);
  const actualTheme = useActualTheme();
  const [homePageContentLoaded, setHomePageContentLoaded] = useState(false);
  const [homePageContent, setHomePageContent] = useState('');
  const [noticeVisible, setNoticeVisible] = useState(false);
  const [activeTab, setActiveTab] = useState('Python');
  const [models, setModels] = useState(chinaFallbackModels);
  const isMobile = useIsMobile();
  const docsLink = statusState?.status?.docs_link || '#';
  const serverAddress =
    statusState?.status?.public_api_base_url ||
    statusState?.status?.server_address ||
    getFallbackServerAddress();
  const endpoint = `${serverAddress.replace(/\/$/, '')}/v1`;
  const codeSnippets = useMemo(() => createCodeSnippets(endpoint), [endpoint]);

  const displayHomePageContent = async () => {
    const cachedContent = localStorage.getItem('home_page_content') || '';
    setHomePageContent(cachedContent);

    try {
      const res = await API.get('/api/home_page_content');
      const { success, message, data } = res.data;
      if (success) {
        if (!data) {
          setHomePageContent('');
          localStorage.removeItem('home_page_content');
          return;
        }

        let content = data;
        if (!data.startsWith('https://')) {
          content = marked.parse(data);
        }
        setHomePageContent(content);
        localStorage.setItem('home_page_content', content);

        if (data.startsWith('https://')) {
          const iframe = document.querySelector('iframe');
          if (iframe) {
            iframe.onload = () => {
              iframe.contentWindow.postMessage({ themeMode: actualTheme }, '*');
              iframe.contentWindow.postMessage({ lang: i18n.language }, '*');
            };
          }
        }
      } else {
        showError(message);
        setHomePageContent('');
      }
    } catch (error) {
      console.error('加载首页内容失败:', error);
      setHomePageContent(cachedContent);
    } finally {
      setHomePageContentLoaded(true);
    }
  };

  const handleCopyEndpoint = async () => {
    const ok = await copy(endpoint);
    if (ok) {
      showSuccess(t('已复制到剪切板'));
    }
  };

  useEffect(() => {
    const checkNoticeAndShow = async () => {
      const lastCloseDate = localStorage.getItem('notice_close_date');
      const today = new Date().toDateString();
      if (lastCloseDate !== today) {
        try {
          const res = await API.get('/api/notice');
          const { success, data } = res.data;
          if (success && data && data.trim() !== '') {
            setNoticeVisible(true);
          }
        } catch (error) {
          console.error('获取公告失败:', error);
        }
      }
    };

    checkNoticeAndShow();
  }, []);

  useEffect(() => {
    displayHomePageContent().then();
  }, []);

  useEffect(() => {
    let alive = true;
    API.get('/api/pricing')
      .then((res) => {
        if (!alive) return;
        const rows = Array.isArray(res.data?.data) ? res.data.data : [];
        const nextModels = rows
          .slice(0, 15)
          .map(toMarqueeModel)
          .filter((model) => model.name);
        setModels(nextModels.length > 0 ? nextModels : chinaFallbackModels);
      })
      .catch(() => {
        if (alive) setModels(chinaFallbackModels);
      });
    return () => {
      alive = false;
    };
  }, []);

  return (
    <div className='w-full overflow-x-hidden'>
      <NoticeModal
        visible={noticeVisible}
        onClose={() => setNoticeVisible(false)}
        isMobile={isMobile}
      />
      {homePageContentLoaded && homePageContent === '' ? (
        <main className='min-h-screen bg-background text-foreground'>
          <Hero
            endpoint={endpoint}
            docsLink={docsLink}
            onCopyEndpoint={handleCopyEndpoint}
          />
          <ModelsMarquee items={models} />
          <Infrastructure />
          <Developer
            snippets={codeSnippets}
            activeTab={activeTab}
            setActiveTab={setActiveTab}
          />
          <Steps />
          <Scenarios />
        </main>
      ) : (
        <div className='overflow-x-hidden w-full'>
          {homePageContent.startsWith('https://') ? (
            <iframe
              src={homePageContent}
              className='w-full h-screen border-none'
            />
          ) : (
            <div
              className='mt-[60px]'
              dangerouslySetInnerHTML={{ __html: homePageContent }}
            />
          )}
        </div>
      )}
    </div>
  );
};

export default Home;
