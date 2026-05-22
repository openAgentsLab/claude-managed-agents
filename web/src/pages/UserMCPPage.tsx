import AppLayout from '@/components/AppLayout'
import MCPPage from '@/components/MCPPage'
import { listMCPServers } from '@/api/mcp'

export default function UserMCPPage() {
  return (
    <AppLayout>
      <MCPPage
        title="MCP Servers"
        description="MCP servers available in your sessions. Configured by your admin."
        listFn={listMCPServers}
        readOnly
      />
    </AppLayout>
  )
}
