// Тесты PipelineLiveLine (S-132b): чистая презентация, без сокетов.

import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'

import { PipelineLiveLine } from './PipelineLiveLine'

describe('PipelineLiveLine', () => {
  it('ничего не рендерит: live без событий', () => {
    const { container } = render(<PipelineLiveLine live={true} notices={[]} />)
    expect(container).toBeEmptyDOMElement()
  })

  it('показывает offline без событий', () => {
    render(<PipelineLiveLine live={false} notices={[]} />)
    expect(screen.getByTestId('pipeline-live')).toHaveTextContent('realtime offline')
  })

  it('показывает последнее событие с деталью', () => {
    render(
      <PipelineLiveLine
        live={true}
        notices={[
          { kind: 'document_generated', title: 'Документ готов', detail: 'spec · pdf', at: 1 },
          { kind: 'pipeline_status', title: 'Конвейер завершён', at: 0 },
        ]}
      />,
    )
    const line = screen.getByTestId('pipeline-live')
    expect(line).toHaveTextContent('●')
    expect(line).toHaveTextContent('live')
    expect(line).toHaveTextContent('Документ готов')
    expect(line).toHaveTextContent('spec · pdf')
  })
})
