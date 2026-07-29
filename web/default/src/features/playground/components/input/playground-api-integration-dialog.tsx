import { ExternalLinkIcon } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import {
  CodeBlock,
  CodeBlockCopyButton,
} from '@/components/ai-elements/code-block'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'

import type { PlaygroundIntegrationGuide } from '../../lib'

type Props = {
  apiBaseUrl: string
  guide: PlaygroundIntegrationGuide
  model: string
  onOpenChange: (open: boolean) => void
  open: boolean
}

const integrationCodeBlockClassName =
  'my-0 [&_.cm-scroller]:overflow-x-auto [&_.cm-scroller]:overflow-y-hidden [&_.code-block-scroll]:overflow-x-auto [&_.code-block-scroll]:overflow-y-hidden'

export function PlaygroundApiIntegrationDialog(props: Props) {
  const { t } = useTranslation()

  return (
    <Dialog open={props.open} onOpenChange={props.onOpenChange}>
      <DialogContent className='flex max-h-[calc(100dvh-1rem)] w-[calc(100vw-1rem)] max-w-none flex-col gap-0 overflow-hidden p-0 sm:max-h-[min(88dvh,54rem)] sm:w-[min(56rem,calc(100vw-3rem))] sm:max-w-4xl'>
        <DialogHeader className='border-border/70 shrink-0 border-b px-4 py-4 pr-12 sm:px-5'>
          <DialogTitle>{t('API integration')}</DialogTitle>
          <DialogDescription className='flex flex-wrap items-center gap-x-2 gap-y-1'>
            <span>{props.model}</span>
            <span aria-hidden='true'>·</span>
            <span className='font-mono text-xs'>{props.apiBaseUrl}</span>
          </DialogDescription>
        </DialogHeader>

        <div className='flex min-h-0 flex-1 flex-col gap-4 overflow-y-auto overscroll-contain px-4 py-4 sm:gap-5 sm:px-5 sm:py-5'>
          {props.guide.overview && (
            <section className='bg-muted/35 border-border/60 rounded-lg border px-4 py-3'>
              <p className='text-sm leading-6'>{props.guide.overview}</p>
              <p className='text-muted-foreground mt-2 text-xs leading-5'>
                {t(
                  'Use Bearer authentication and keep the API key on your server.'
                )}
              </p>
            </section>
          )}

          <div className='flex flex-col gap-4 sm:gap-5'>
            {props.guide.interfaces.map((item, index) => (
              <section
                key={item.key}
                className='border-border/70 overflow-hidden rounded-xl border'
              >
                <div className='bg-muted/25 border-border/60 flex flex-col gap-2 border-b px-4 py-3'>
                  <div className='flex flex-wrap items-center gap-2'>
                    <span className='text-muted-foreground text-xs font-medium'>
                      {t('Interface {{number}}', { number: index + 1 })}
                    </span>
                    <Badge variant='outline'>{item.method}</Badge>
                    <code className='text-foreground min-w-0 text-xs break-all'>
                      {item.path}
                    </code>
                  </div>
                  <h3 className='text-sm font-semibold'>{item.title}</h3>
                  {item.description && (
                    <p className='text-muted-foreground text-sm leading-6'>
                      {item.description}
                    </p>
                  )}
                  {item.requestDescription && (
                    <p className='text-muted-foreground text-xs leading-5'>
                      {item.requestDescription}
                    </p>
                  )}
                </div>

                <div className='flex flex-col gap-4 px-4 py-4'>
                  <div className='flex flex-col gap-2'>
                    <h4 className='text-xs font-medium'>
                      {t('Request example')}
                    </h4>
                    <CodeBlock
                      className={integrationCodeBlockClassName}
                      collapsedLines={10}
                      code={item.curl}
                      defaultCollapsed
                      language='bash'
                      showToolbar
                    >
                      <CodeBlockCopyButton />
                    </CodeBlock>
                  </div>

                  {item.responseExample && (
                    <div className='flex flex-col gap-2'>
                      <h4 className='text-xs font-medium'>
                        {t('Response example')}
                      </h4>
                      <CodeBlock
                        className={integrationCodeBlockClassName}
                        collapsedLines={8}
                        code={item.responseExample}
                        defaultCollapsed
                        language='json'
                        showToolbar
                      >
                        <CodeBlockCopyButton />
                      </CodeBlock>
                    </div>
                  )}

                  {item.notes.length > 0 && (
                    <ul className='text-muted-foreground flex list-disc flex-col gap-1 pl-5 text-xs leading-5'>
                      {item.notes.map((note) => (
                        <li key={note}>{note}</li>
                      ))}
                    </ul>
                  )}
                </div>
              </section>
            ))}
          </div>

          {props.guide.completeExample && (
            <section className='flex flex-col gap-2'>
              <div>
                <h3 className='text-sm font-semibold'>
                  {t('Complete workflow')}
                </h3>
                <p className='text-muted-foreground mt-1 text-xs leading-5'>
                  {t(
                    'This example includes the calling order and can be copied as a starting point for integration.'
                  )}
                </p>
              </div>
              <CodeBlock
                className={integrationCodeBlockClassName}
                collapsedLines={10}
                code={props.guide.completeExample}
                defaultCollapsed
                language='bash'
                showToolbar
              >
                <CodeBlockCopyButton />
              </CodeBlock>
            </section>
          )}

          {props.guide.resultNote && (
            <section className='border-border/60 rounded-lg border px-4 py-3'>
              <h3 className='text-sm font-semibold'>{t('Result handling')}</h3>
              <p className='text-muted-foreground mt-1 text-xs leading-5'>
                {props.guide.resultNote}
              </p>
            </section>
          )}

          {props.guide.documentationUrl && (
            <Button
              render={
                <a
                  href={props.guide.documentationUrl}
                  rel='noreferrer'
                  target='_blank'
                />
              }
              size='sm'
              variant='outline'
            >
              {t('View API documentation')}
              <ExternalLinkIcon className='size-3.5' />
            </Button>
          )}
        </div>
      </DialogContent>
    </Dialog>
  )
}
