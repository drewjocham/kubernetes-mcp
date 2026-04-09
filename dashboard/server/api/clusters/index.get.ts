import { listClusters } from '~/server/utils/cluster-store'

export default defineEventHandler(async () => {
  const clusters = listClusters()
  return { clusters }
})