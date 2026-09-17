import { createFileRoute } from '@tanstack/react-router'
import { Main } from '@/components/layout'
import { ResearchWorkbench } from '@/features/research'

export const Route = createFileRoute('/_authenticated/research/')({
  component: ResearchPage,
})

function ResearchPage() {
  return (
    <Main className="p-0">
      <ResearchWorkbench />
    </Main>
  )
}
