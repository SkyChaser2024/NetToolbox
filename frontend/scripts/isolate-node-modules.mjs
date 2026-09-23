import { writeFileSync } from 'node:fs'

// Some npm packages bundle Go examples. A nested module keeps `go test ./...`
// and `go vet ./...` scoped to this application's code after npm ci/install.
try {
  writeFileSync(
    new URL('../node_modules/go.mod', import.meta.url),
    'module nettoolbox-frontend-dependencies\n',
    { flag: 'wx' }
  )
} catch (error) {
  if (error.code !== 'EEXIST') throw error
}
