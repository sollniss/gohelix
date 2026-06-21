# gohelix

A terminal game for practicing [Helix](https://helix-editor.com) motions. Each
challenge gives you a `before.go` and an `after.go`. Your job is to transform one
into the other in as few keystrokes as possible. Your best keystroke count and time are saved
per challenge.

<video src="https://github.com/sollniss/gohelix/raw/main/docs/demo.webm" controls muted width="800"></video>

> If the video doesn't play above, [watch demo.webm](docs/demo.webm).

## Install

Try it without installing:

```sh
go run github.com/sollniss/gohelix@latest
```

Or install it:

```sh
go install github.com/sollniss/gohelix@latest
```

Or build from source:

```sh
go build -o gohelix .
```

Requires `hx` (Helix) on your `PATH`. [`bat`](https://github.com/sharkdp/bat) and
[`difftastic`](https://difftastic.wilfred.me.uk) are optional and improve the
diff view if present.

## Usage

```sh
gohelix          # start a random challenge
gohelix -c 9     # jump straight to challenge 9
```
