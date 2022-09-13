More information can be found here https://www.sqlite.org/howtocompile.html

## Linux

### Download source files

```
wget https://www.sqlite.org/2022/sqlite-amalgamation-3390200.zip
unzip -j -d src sqlite-amalgamation-3390200.zip
wget -O src/dbhash.c https://raw.githubusercontent.com/sqlite/sqlite/master/tool/dbhash.c
```

### Compile

Examples of compilation with different options
```
cd src
gcc -O2 dbhash.c sqlite3.c -lpthread -ldl -lm -o dbhash
gcc -O2 -DSQLITE_OMIT_LOAD_EXTENSION dbhash.c sqlite3.c -lpthread -lm -o dbhash
```

## Cross compilation

```
mkdir -p dist/linux_amd64
docker run -it --rm -v $(pwd):/workdir --workdir /workdir/src --user $UID:$GID multiarch/crossbuild gcc -O2 -DSQLITE_THREADSAFE=0 -DSQLITE_OMIT_LOAD_EXTENSION dbhash.c sqlite3.c -lm -o ../dist/linux_amd64/dbhash
```

```
mkdir -p dist/windows_amd64
docker run -it --rm -v $(pwd):/workdir -e CROSS_TRIPLE=x86_64-w64-mingw32 --workdir /workdir/src --user $UID:$GID multiarch/crossbuild gcc -O2 -DSQLITE_THREADSAFE=0 -DSQLITE_OMIT_LOAD_EXTENSION dbhash.c sqlite3.c -lm -o ../dist/windows_amd64/dbhash.exe
```

```
mkdir -p dist/darwin_amd64
docker run -it --rm -v $(pwd):/workdir -e CROSS_TRIPLE=x86_64-apple-darwin --workdir /workdir/src --user $UID:$GID multiarch/crossbuild gcc -O2 -DSQLITE_THREADSAFE=0 -DSQLITE_OMIT_LOAD_EXTENSION dbhash.c sqlite3.c -lm -o ../dist/darwin_amd64/dbhash
```
