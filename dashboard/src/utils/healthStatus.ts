export function healthBadge(props: { status?: string; available?: boolean; failCount?: number; enabled?: boolean }) {
  if (props.enabled === false) return { label: '已停用', tone: 'muted' as const, detail: '该图源未参与分发。' }
  if (props.status === 'error') return { label: '暂停分发', tone: 'warning' as const, detail: '近期取图失败较多，已暂时停止分发。可重新检测图源。' }
  if (props.available === false) return { label: '本次检测失败', tone: 'warning' as const, detail: '本次请求未通过，可能与超时、鉴权、代理或响应格式有关。请查看检测详情。' }
  if (props.available === true) return { label: '检测通过', tone: 'success' as const, detail: '最近一次检测至少有一个地址通过，不保证所有图片都能下载。' }
  if ((props.failCount ?? 0) > 0) return { label: '最近请求失败', tone: 'warning' as const, detail: '存在失败记录，尚无有效检测结果。' }
  return { label: '未检测', tone: 'muted' as const, detail: '暂无有效检测结果。' }
}

export function isHealthResultCurrent(checkedAt: string | undefined, updatedAt: string) {
  if (!checkedAt) return false
  const checked = Date.parse(checkedAt)
  const updated = Date.parse(updatedAt.includes('T') ? updatedAt : updatedAt.replace(' ', 'T') + 'Z')
  return Number.isFinite(checked) && Number.isFinite(updated) && checked >= updated
}
