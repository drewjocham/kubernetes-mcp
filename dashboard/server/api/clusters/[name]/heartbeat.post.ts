import { updateClusterHeartbeat } from '~/server/utils/cluster-store'

export default defineEventHandler(async (event) => {
  const name = getRouterParam(event, 'name')
  
  if (!name) {
    throw createError({
      statusCode: 400,
      statusMessage: 'Cluster name is required'
    })
  }
  
  const cluster = updateClusterHeartbeat(name)
  
  if (!cluster) {
    throw createError({
      statusCode: 404,
      statusMessage: 'Cluster not found'
    })
  }
  
  return {
    ok: true,
    cluster
  }
})