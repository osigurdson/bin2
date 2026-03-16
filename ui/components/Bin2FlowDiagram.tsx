import type { ReactNode } from 'react';
import { ArrowDown, Globe, LaptopMinimal, Package2, Server } from 'lucide-react';

const tones = {
  info: {
    icon: 'border-info/20 bg-info/10 text-info',
    pill: 'border-info/20 bg-info/10 text-info',
    card: 'border-info/40',
    stroke: 'stroke-info',
    fill: 'fill-info',
  },
  secondary: {
    icon: 'border-secondary/20 bg-secondary/10 text-secondary',
    pill: 'border-secondary/20 bg-secondary/10 text-secondary',
    card: 'border-secondary/40',
    stroke: 'stroke-secondary',
    fill: 'fill-secondary',
  },
  success: {
    icon: 'border-success/20 bg-success/10 text-success',
    pill: 'border-success/20 bg-success/10 text-success',
    card: 'border-success/40',
    stroke: 'stroke-success',
    fill: 'fill-success',
  },
} as const;

type Tone = keyof typeof tones;

type NodeCardProps = {
  title: ReactNode;
  description: string;
  tone: Tone;
  badge?: string;
  icon: ReactNode;
  className?: string;
};

type FlowLabelProps = {
  tone: Tone;
  children: ReactNode;
  className?: string;
};

export default function Bin2FlowDiagram() {
  return (
    <div className='flex flex-col border border-base-200 bg-base-100 p-5 sm:p-6'>
      <style>{`
        @keyframes flow-fwd { to { stroke-dashoffset: -20; } }
        @keyframes flow-rev { to { stroke-dashoffset:  20; } }
      `}</style>

      {/* Mobile layout */}
      <div className='flex flex-col gap-3 md:hidden'>
        <NodeCard
          title='localhost'
          description='dev / CI'
          tone='info'
          icon={<LaptopMinimal size={20} strokeWidth={1.75} />}
        />
        <div className='flex flex-wrap gap-2 px-2'>
          <FlowLabel tone='info'>push</FlowLabel>
          <FlowLabel tone='secondary'>pull</FlowLabel>
        </div>
        <div className='flex justify-center text-base-content/30'>
          <ArrowDown size={18} strokeWidth={1.75} />
        </div>
        <NodeCard
          title={<>bin<sub>2</sub>.io</>}
          description='origin site'
          tone='info'
          badge='push + pull'
          icon={<Package2 size={20} strokeWidth={1.75} />}
        />
        <div className='flex justify-center text-base-content/20'>
          <ArrowDown size={18} strokeWidth={1.75} />
        </div>
        <NodeCard
          title={<>pull.bin<sub>2</sub>.io</>}
          description='global CDN'
          tone='success'
          badge='pull only'
          icon={<Globe size={20} strokeWidth={1.75} />}
        />
        <div className='flex flex-wrap gap-2 px-2'>
          <FlowLabel tone='success'>CDN pull</FlowLabel>
        </div>
        <div className='flex justify-center text-base-content/30'>
          <ArrowDown size={18} strokeWidth={1.75} />
        </div>
        <NodeCard
          title='k8s / prod'
          description='workloads'
          tone='success'
          icon={<Server size={20} strokeWidth={1.75} />}
        />
      </div>

      {/* Desktop layout
          Left column:  localhost (top) · k8s (bottom)
          Right column: bin2.io (top)   · pull.bin2.io (bottom)
          Fixed 580×440 so CSS px == SVG viewBox units. */}
      <div className='relative hidden h-[440px] w-[580px] md:block mx-auto' aria-hidden='true'>
        <svg className='absolute inset-0 h-full w-full' viewBox='0 0 580 440' fill='none'>
          <defs>
            <marker id='flow-arrow-info' markerWidth='8' markerHeight='8' refX='7' refY='4' orient='auto'>
              <path d='M 0 0 L 8 4 L 0 8 z' className={tones.info.fill} />
            </marker>
            <marker id='flow-arrow-secondary' markerWidth='8' markerHeight='8' refX='7' refY='4' orient='auto'>
              <path d='M 0 0 L 8 4 L 0 8 z' className={tones.secondary.fill} />
            </marker>
            <marker id='flow-arrow-success' markerWidth='8' markerHeight='8' refX='7' refY='4' orient='auto'>
              <path d='M 0 0 L 8 4 L 0 8 z' className={tones.success.fill} />
            </marker>
          </defs>

          {/* push: localhost → bin2.io (animated) */}
          <path
            d='M 175 92 L 370 104'
            className={tones.info.stroke}
            strokeWidth='2'
            strokeDasharray='6 4'
            markerEnd='url(#flow-arrow-info)'
            style={{ animation: 'flow-fwd 1.6s linear infinite' }}
          />
          {/* direct pull: bin2.io → localhost (static) */}
          <path
            d='M 370 150 L 175 126'
            className={tones.secondary.stroke}
            strokeWidth='2'
            strokeDasharray='6 4'
            markerEnd='url(#flow-arrow-secondary)'
          />
          {/* bin2.io ↔ pull.bin2.io: static dotted connector */}
          <path
            d='M 457 202 L 457 260'
            className='stroke-base-content/25'
            strokeWidth='1.5'
            strokeDasharray='3 5'
          />
          {/* CDN pull: pull.bin2.io → k8s (animated, flows right-to-left) */}
          <path
            d='M 370 341 L 175 334'
            className={tones.success.stroke}
            strokeWidth='2'
            strokeDasharray='6 4'
            markerEnd='url(#flow-arrow-success)'
            style={{ animation: 'flow-fwd 1.6s linear infinite' }}
          />
        </svg>

        {/* localhost — top left */}
        <div className='absolute left-[15px] top-[40px]'>
          <NodeCard
            title='localhost'
            description='dev / CI'
            tone='info'
            icon={<LaptopMinimal size={20} strokeWidth={1.75} />}
            className='w-[160px]'
          />
        </div>

        {/* k8s cluster — bottom left */}
        <div className='absolute left-[15px] top-[270px]'>
          <NodeCard
            title='k8s / prod'
            description='workloads'
            tone='success'
            icon={<Server size={20} strokeWidth={1.75} />}
            className='w-[160px]'
          />
        </div>

        {/* bin2.io — top right */}
        <div className='absolute left-[370px] top-[40px]'>
          <NodeCard
            title={<>bin<sub>2</sub>.io</>}
            description='origin site'
            tone='info'
            badge='push + pull'
            icon={<Package2 size={20} strokeWidth={1.75} />}
            className='w-[175px]'
          />
        </div>

        {/* pull.bin2.io — bottom right */}
        <div className='absolute left-[370px] top-[260px]'>
          <NodeCard
            title={<>pull.bin<sub>2</sub>.io</>}
            description='global CDN'
            tone='success'
            badge='pull only'
            icon={<Globe size={20} strokeWidth={1.75} />}
            className='w-[175px]'
          />
        </div>

        <FlowLabel tone='info' className='absolute left-[255px] top-[70px]'>
          push
        </FlowLabel>
        <FlowLabel tone='secondary' className='absolute left-[255px] top-[142px]'>
          pull
        </FlowLabel>
        <FlowLabel tone='success' className='absolute left-[243px] top-[341px]'>
          CDN pull
        </FlowLabel>
      </div>
    </div>
  );
}

function NodeCard({ title, description, tone, badge, icon, className = '' }: NodeCardProps) {
  return (
    <div className={`flex flex-col items-center gap-3 rounded-2xl border bg-base-100 p-5 text-center ${tones[tone].card} ${className}`.trim()}>
      <div className={`flex h-11 w-11 items-center justify-center rounded-xl border ${tones[tone].icon}`}>
        {icon}
      </div>
      <div className='flex flex-col gap-1'>
        <p className='font-semibold leading-tight text-base-content'>{title}</p>
        <p className='text-sm text-base-content/50'>{description}</p>
      </div>
      {badge ? (
        <span className={`rounded border px-2 py-1 text-[10px] font-medium uppercase tracking-[1.5px] ${tones[tone].pill}`}>
          {badge}
        </span>
      ) : null}
    </div>
  );
}

function FlowLabel({ tone, children, className = '' }: FlowLabelProps) {
  return (
    <span className={`rounded border px-2 py-1 text-[10px] font-medium uppercase tracking-[1.5px] ${tones[tone].pill} ${className}`.trim()}>
      {children}
    </span>
  );
}
