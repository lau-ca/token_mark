/*
Copyright (C) 2023-2026 QuantumNous

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
import type { Column } from '@tanstack/react-table'
import { Check as CheckIcon, PlusCircle as PlusCircledIcon } from 'lucide-react'
import * as React from 'react'
import { useTranslation } from 'react-i18next'

import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Command,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
  CommandSeparator,
} from '@/components/ui/command'
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from '@/components/ui/popover'
import { Separator } from '@/components/ui/separator'
import { cn } from '@/lib/utils'

export type FacetedFilterOption = {
  label: string
  value: string
  icon?: React.ComponentType<{ className?: string }>
  iconNode?: React.ReactNode
  count?: number
}

type FacetedFilterProps = {
  title?: string
  options: FacetedFilterOption[]
  selectedValues?: string[]
  onSelectedValuesChange: (values: string[]) => void
  singleSelect?: boolean
  facets?: Map<unknown, number>
}

function FacetedFilterInner(props: FacetedFilterProps) {
  const { t } = useTranslation()
  const selectedValues = new Set(props.selectedValues)

  const handleOptionSelect = (optionValue: string) => {
    const nextSelectedValues = getNextSelectedValues(
      selectedValues,
      optionValue,
      props.singleSelect ?? false
    )

    props.onSelectedValuesChange(nextSelectedValues)
  }

  return (
    <Popover>
      <PopoverTrigger
        render={
          <Button variant='outline' size='sm' className='h-8 border-dashed' />
        }
      >
        <PlusCircledIcon className='size-4' />
        {props.title}
        {selectedValues?.size > 0 && (
          <>
            <Separator orientation='vertical' className='mx-2 h-4' />
            <Badge
              variant='secondary'
              className='rounded-sm px-1 font-normal lg:hidden'
            >
              {selectedValues.size}
            </Badge>
            <div className='hidden space-x-1 lg:flex'>
              {selectedValues.size > 2 ? (
                <Badge
                  variant='secondary'
                  className='rounded-sm px-1 font-normal'
                >
                  {selectedValues.size} {t('selected')}
                </Badge>
              ) : (
                props.options
                  .filter((option) => selectedValues.has(option.value))
                  .map((option) => (
                    <Badge
                      variant='secondary'
                      key={option.value}
                      className='rounded-sm px-1 font-normal'
                    >
                      {t(option.label)}
                    </Badge>
                  ))
              )}
            </div>
          </>
        )}
      </PopoverTrigger>
      <PopoverContent className='max-w-[360px] min-w-[200px] p-0' align='start'>
        <Command>
          <CommandInput placeholder={props.title} />
          <CommandList>
            <CommandEmpty>{t('No results found.')}</CommandEmpty>
            <CommandGroup>
              {props.options.map((option) => {
                const isSelected = selectedValues.has(option.value)
                let optionIcon: React.ReactNode = null
                if (option.iconNode) {
                  optionIcon = (
                    <span className='text-muted-foreground flex size-4 items-center justify-center'>
                      {option.iconNode}
                    </span>
                  )
                } else if (option.icon) {
                  optionIcon = (
                    <option.icon className='text-muted-foreground size-4' />
                  )
                }

                let optionCount: React.ReactNode = null
                if (typeof option.count === 'number') {
                  optionCount = (
                    <span className='text-muted-foreground ms-auto flex h-4 min-w-4 items-center justify-center font-mono text-xs'>
                      {option.count}
                    </span>
                  )
                } else {
                  const facetCount = props.facets?.get(option.value)
                  if (facetCount) {
                    optionCount = (
                      <span className='ms-auto flex h-4 w-4 items-center justify-center font-mono text-xs'>
                        {facetCount}
                      </span>
                    )
                  }
                }

                return (
                  <CommandItem
                    key={option.value}
                    onSelect={() => handleOptionSelect(option.value)}
                  >
                    <div
                      className={cn(
                        'border-primary flex size-4 items-center justify-center rounded-sm border',
                        isSelected
                          ? 'bg-primary text-primary-foreground'
                          : 'opacity-50 [&_svg]:invisible'
                      )}
                    >
                      <CheckIcon className={cn('text-background h-4 w-4')} />
                    </div>
                    {optionIcon}
                    <span
                      className='min-w-0 flex-1 truncate'
                      title={t(option.label)}
                    >
                      {t(option.label)}
                    </span>
                    {optionCount}
                  </CommandItem>
                )
              })}
            </CommandGroup>
            {selectedValues.size > 0 && (
              <>
                <CommandSeparator />
                <CommandGroup>
                  <CommandItem
                    onSelect={() => props.onSelectedValuesChange([])}
                    className='justify-center text-center'
                  >
                    {t('Clear filters')}
                  </CommandItem>
                </CommandGroup>
              </>
            )}
          </CommandList>
        </Command>
      </PopoverContent>
    </Popover>
  )
}

export const FacetedFilter = React.memo(FacetedFilterInner)

type DataTableFacetedFilterProps<TData, TValue> = {
  column?: Column<TData, TValue>
  title?: string
  options: FacetedFilterOption[]
  singleSelect?: boolean
}

function DataTableFacetedFilterInner<TData, TValue>(
  props: DataTableFacetedFilterProps<TData, TValue>
) {
  const filterValue = props.column?.getFilterValue() as string[] | undefined

  return (
    <FacetedFilter
      title={props.title}
      options={props.options}
      selectedValues={filterValue}
      onSelectedValuesChange={(values) =>
        props.column?.setFilterValue(values.length ? values : undefined)
      }
      singleSelect={props.singleSelect}
      facets={props.column?.getFacetedUniqueValues()}
    />
  )
}

export const DataTableFacetedFilter = React.memo(
  DataTableFacetedFilterInner
) as typeof DataTableFacetedFilterInner

function getNextSelectedValues(
  selectedValues: Set<string>,
  optionValue: string,
  singleSelect: boolean
): string[] {
  if (singleSelect) {
    return selectedValues.has(optionValue) ? [] : [optionValue]
  }

  const nextSelectedValues = new Set(selectedValues)
  if (nextSelectedValues.has(optionValue)) {
    nextSelectedValues.delete(optionValue)
  } else {
    nextSelectedValues.add(optionValue)
  }

  return [...nextSelectedValues]
}
