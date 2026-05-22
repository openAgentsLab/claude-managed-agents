import AppLayout from '@/components/AppLayout'
import SkillsPage from '@/components/SkillsPage'
import { listSkills, getSkill } from '@/api/skills'

export default function UserSkillsPage() {
  return (
    <AppLayout>
      <SkillsPage
        title="Skills"
        description="Skills available in your sessions. Configured by your admin."
        listFn={listSkills}
        getFn={getSkill}
        readOnly
      />
    </AppLayout>
  )
}
