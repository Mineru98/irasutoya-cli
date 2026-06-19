# irasutoya-cli

언어: [English](./README.md) | [日本語](./README.ja.md) | [中文](./README.zh.md) | **한국어**

![irasutoya-cli 한국어 데모](./images/demo-ko.gif)

[![Libraries.io dependency status for GitHub repo](https://img.shields.io/librariesio/github/Mineru98/irasutoya-cli.svg)](https://libraries.io/github/Mineru98/irasutoya-cli)
![GitHub](https://img.shields.io/github/license/Mineru98/irasutoya-cli.svg)

## Claude Code / Codex 설치

이 저장소는 Claude Code **플러그인 마켓플레이스**이자, 독립 실행형 Claude/Codex 스킬도 함께 제공합니다. 모두 실제 `irasutoya` CLI 검색 래퍼를 실행합니다. 프로젝트 스킬은 `.claude/skills/<스킬명>/SKILL.md`에, 플러그인 스킬은 `<플러그인>/skills/<스킬명>/SKILL.md`에 위치하며 이는 Claude Code 스킬·플러그인 문서 규약을 따릅니다. 워크플로에 맞는 방법을 선택하세요.

### Claude 플러그인 — 마켓플레이스 (권장)

Claude Code 플러그인을 설치하는 표준 방식입니다. 저장소를 마켓플레이스로 추가한 뒤, 거기서 플러그인을 설치합니다.

```text
/plugin marketplace add Mineru98/irasutoya-cli
/plugin install irasutoya-search@irasutoya-cli
```

셸에서 비대화형으로 실행할 수도 있습니다.

```sh
claude plugin marketplace add Mineru98/irasutoya-cli
claude plugin install irasutoya-search@irasutoya-cli
```

설치 후 네임스페이스가 붙은 스킬을 호출합니다.

```text
/irasutoya-search:irasutoya-search cat
```

### Claude 플러그인 — 로컬 디렉터리 (개발용)

마켓플레이스 없이 로컬 체크아웃에서 바로 플러그인을 로드합니다.

```sh
claude --plugin-dir .claude/plugins/irasutoya-search
```

호출 방식은 동일합니다(`/irasutoya-search:irasutoya-search cat`). 실행 중인 세션에서 플러그인 파일을 수정했다면 `/reload-plugins`를 실행하거나 Claude Code를 재시작하세요.

### Claude 프로젝트 스킬 (플러그인 없이)

저장소 루트에서 Claude Code를 실행하면 `.claude/skills/irasutoya-search`를 자동으로 인식합니다.

```sh
claude
```

네임스페이스 없는 스킬을 호출합니다.

```text
/irasutoya-search cat
```

### Codex 스킬

저장소에 포함된 프로젝트 로컬 Codex 스킬을 사용합니다.

```sh
python .codex/skills/irasutoya-search/scripts/irasutoya_search.py cat
```

Codex에서는 `$irasutoya-search`로 호출하거나 자연어로 Irasutoya 일러스트 검색을 요청하세요.

### 플러그인 로드 검증

배포 전이나 변경 후에 매니페스트를 검증하고 등록된 스킬을 확인합니다.

```sh
# 마켓플레이스와 플러그인 매니페스트 검증 (CI에서는 --strict 사용)
claude plugin validate .claude-plugin/marketplace.json --strict
claude plugin validate .claude/plugins/irasutoya-search --strict

# 설치 후 스킬이 등록되었는지 확인
claude plugin list
claude plugin details irasutoya-search
```

`claude plugin details irasutoya-search` 출력에 `Skills (1) irasutoya-search`가 표시되어야 합니다.

## 설치

네이티브 Go CLI는 Windows, macOS, Linux를 위한 크로스 플랫폼 배포 대상입니다.

```sh
$ git clone https://github.com/Mineru98/irasutoya-cli.git
$ cd irasutoya-cli
$ go build ./cmd/irasutoya
```

CI와 릴리스 기준은 Go 1.26.4입니다. 현재 `go.mod`는 로컬 마이그레이션 환경의 Go 1.24.3 툴체인과도 호환되며, 로컬 툴체인이 업그레이드될 때까지 이 호환성을 유지합니다.

## 사용법

```sh
$ irasutoya help
Commands:
  irasutoya random          # 무작위 irasutoya 이미지를 표시합니다
  irasutoya search {query}  # 검색어로 이미지 3개를 표시합니다
```

CLI는 ONE PIECE 캐릭터 데모에 쓰이는 다국어 검색어를 지원합니다. 예를 들어 `luffy`, `zoro`, `ルフィ`, `ゾロ`, `路飞`, `索隆`, `루피`, `조로`를 사용할 수 있습니다.

기본적으로 Go CLI는 페이지 메타데이터와 이미지 URL만 출력하며 외부 앱을 열지 않습니다. OS 기본 앱으로 이미지 URL을 열려면 명시적으로 활성화하세요.

```sh
$ irasutoya --open-images random
$ IRASUTOYA_OPEN_IMAGES=1 irasutoya search 루피
```

## 개발

```sh
$ go test ./...
$ go build ./cmd/irasutoya
```

릴리스 빌드는 GoReleaser를 사용하며 `CGO_ENABLED=0`으로 Windows, macOS, Linux 아카이브를 만듭니다.

```sh
$ goreleaser check
$ goreleaser release --snapshot --clean
```

## 기여

이 포크의 버그 리포트와 변경 사항은 GitHub의 https://github.com/Mineru98/irasutoya-cli 에서 다룹니다. 이 프로젝트는 안전하고 환영받는 협업 공간을 지향하며, 기여자는 [Contributor Covenant](http://contributor-covenant.org) 행동 강령을 따라야 합니다.

## 라이선스

이 프로젝트는 [MIT License](https://opensource.org/licenses/MIT) 조건에 따라 오픈 소스로 제공됩니다.

## 행동 강령

irasutoya-cli 프로젝트의 코드베이스, 이슈 트래커, 채팅방, 메일링 리스트에 참여하는 모든 사람은 [행동 강령](https://github.com/Mineru98/irasutoya-cli/blob/master/CODE_OF_CONDUCT.md)을 따라야 합니다.

## 작성자

포크 유지보수: [@Mineru98](https://github.com/Mineru98)

원본 프로젝트: [@unhappychoice](https://unhappychoice.com)
