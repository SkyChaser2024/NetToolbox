import { readFileSync } from 'node:fs'

export function verifyVersions(versions, tag) {
  const { frontend, backend, windows } = versions
  if (!frontend || frontend !== backend || frontend !== windows) {
    throw new Error(`版本不一致：${JSON.stringify(versions)}`)
  }
  if (tag !== undefined && tag !== `v${frontend}`) {
    throw new Error(`发布标签 ${tag} 与应用版本 v${frontend} 不一致`)
  }
  return frontend
}

if (import.meta.main) {
  try {
    const read = (path) => readFileSync(new URL(path, import.meta.url), 'utf8')
    const version = verifyVersions(
      {
        frontend: JSON.parse(read('../package.json')).version,
        windows: JSON.parse(read('../../wails.json')).info.productVersion,
        backend: read('../../internal/appmeta/appmeta.go').match(
          /^\s*Version\s*=\s*"([^"]+)"/m
        )?.[1]
      },
      process.env.GITHUB_REF_TYPE === 'tag' ? process.env.GITHUB_REF_NAME : undefined
    )
    console.log(`应用版本一致：${version}`)
  } catch (error) {
    console.error(error.message)
    process.exitCode = 1
  }
}
