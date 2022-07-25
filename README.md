# QGIS plugin for Gisquick platform

```

## Development

### Build plugin's library

```
cd go
go build -ldflags="-s -w" -buildmode=c-shared -o ../python/gisquick.so cmd/main.go
```

### Plugin development (Linux):
```
ln -s `pwd`/python ~/.local/share/QGIS/QGIS3/profiles/default/python/plugins/gisquick
```
