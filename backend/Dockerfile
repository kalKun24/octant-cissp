# API コンテナ（Cloud Run 用）。ビルドコンテキストは backend/ を想定する。
#   docker build -t octant-api ./backend

# ---- ビルドステージ ----
FROM golang:1.26-bookworm AS builder

WORKDIR /src

# 依存だけ先に解決してレイヤキャッシュを効かせる。ソース変更だけなら再ダウンロードしない。
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# CGO を切って静的リンクにする。distroless/static は libc を持たないため必須。
# -trimpath はバイナリからビルド環境の絶対パスを除き、再現性を上げる。
# -s -w はデバッグ情報を落としてイメージを小さくする。
ENV CGO_ENABLED=0 GOOS=linux
RUN go build -trimpath -ldflags="-s -w" -o /out/server ./cmd/server

# ---- 実行ステージ ----
# distroless/static: シェルもパッケージマネージャも含まないため攻撃面が小さい。
# :nonroot タグは uid/gid 65532 の非 root ユーザーで動く。
FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /

COPY --from=builder /out/server /server

# 待ち受けポートは Cloud Run が PORT で注入する。ここでの EXPOSE と既定値は
# ローカル実行時の目安であり、config.Load() が PORT を優先して読む。
ENV PORT=8080
EXPOSE 8080

USER nonroot:nonroot

ENTRYPOINT ["/server"]
