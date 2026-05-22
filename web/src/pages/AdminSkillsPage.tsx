import AppLayout from '@/components/AppLayout'
import SkillsPage from '@/components/SkillsPage'
import { listTenantSkills, getTenantSkill, upsertTenantSkill, deleteTenantSkill } from '@/api/skills'

export default function AdminSkillsPage() {
  return (
    <AppLayout>
      <SkillsPage
        title="Tenant Skills"
        description="Shared skills available to all users in this tenant. Users can override individual skills."
        listFn={listTenantSkills}
        getFn={getTenantSkill}
        upsertFn={upsertTenantSkill}
        deleteFn={deleteTenantSkill}
      />
    </AppLayout>
  )
}
