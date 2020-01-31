[![GoDoc](https://godoc.org/paste.run?status.svg)](https://godoc.org/paste.run)

# go-client
Go client API for paste.run (golang)

```go
import "paste.run"
```

Upload paste:

```go
pasteURL, err := paste.Upload(r,
	paste.Title("My Paste"),
	paste.Author("Chris"),
	paste.Type("Go"),
)
```

Get paste:

```go
pinfo, err := paste.Get(pasteURL)
```

[PasteInfo](https://godoc.org/paste.run#PasteInfo)
