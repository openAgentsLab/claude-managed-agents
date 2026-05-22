import AppLayout from '@/components/AppLayout'
import MCPPage from '@/components/MCPPage'
import { listTenantMCPServers, upsertTenantMCPServer, deleteTenantMCPServer } from '@/api/mcp'

export default function AdminMCPPage() {
  return (
    <AppLayout>
      <MCPPage
        title="Tenant MCP Servers"
        description="Shared MCP servers available to all users in this tenant. Users can override or disable individual servers."
        listFn={listTenantMCPServers}
        upsertFn={upsertTenantMCPServer}
        deleteFn={deleteTenantMCPServer}
      />
    </AppLayout>
  )
}
