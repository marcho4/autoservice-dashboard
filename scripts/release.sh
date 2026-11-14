#!/usr/bin/env bash
# Выпуск версии: ставит тег vX.Y.Z на текущий коммит main и пушит его.
# По тегу GitHub Actions (.github/workflows/release.yml) прогоняет тесты и собирает образы в GHCR.
#
#   ./scripts/release.sh v1.2.3
set -euo pipefail
cd "$(dirname "$0")/.."

die() { echo "release: $*" >&2; exit 1; }

version=${1:-}
[[ $version =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]] || die "укажите версию вида v1.2.3 (последняя: $(git describe --tags --abbrev=0 2>/dev/null || echo нет))"
[[ -z $(git status --porcelain) ]] || die "есть незакоммиченные изменения"
[[ $(git rev-parse --abbrev-ref HEAD) == main ]] || die "версии выпускаются из main"

git fetch --quiet --tags origin main
[[ $(git rev-parse HEAD) == $(git rev-parse origin/main) ]] || die "локальный main расходится с origin/main"
git rev-parse -q --verify "refs/tags/$version" >/dev/null && die "тег $version уже есть"

git tag -a "$version" -m "$version"
git push origin "$version"

repo=$(git remote get-url origin | sed -E 's#(\.git)?$##; s#.*[:/]([^/]+/[^/]+)$#\1#')
echo "Тег $version отправлен. Сборка: https://github.com/$repo/actions"
echo "Когда она позеленеет: ./scripts/deploy.sh $version"
