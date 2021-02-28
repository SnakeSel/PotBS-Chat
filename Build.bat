@echo off
clear

set proj=potbs_chat
set today=%date:~6,4%%date:~3,2%%date:~0,2%
set mingw=C:\msys64\mingw64
set sevenz="C:\Program Files\7-Zip\7z.exe"
set builddir=%CD%\Build\%today%\%proj%
set libdir=%builddir%
set libs=libatk-1.0-0.dll libbz2-1.dll libcairo-2.dll libcairo-gobject-2.dll libepoxy-0.dll libexpat-1.dll libffi-6.dll libfontconfig-1.dll libfreetype-6.dll libgcc_s_seh-1.dll libgdk-3-0.dll libgdk_pixbuf-2.0-0.dll libgio-2.0-0.dll libgit2.dll libglib-2.0-0.dll libgmodule-2.0-0.dll libgobject-2.0-0.dll libgraphite2.dll libgtk-3-0.dll libharfbuzz-0.dll libiconv-2.dll libintl-8.dll libpango-1.0-0.dll libpangocairo-1.0-0.dll libpangoft2-1.0-0.dll libpangowin32-1.0-0.dll libpcre-1.dll libpixman-1-0.dll libpng16-16.dll libstdc++-6.dll libwinpthread-1.dll zlib1.dll libfribidi-0.dll libthai-0.dll libdatrie-1.dll libffi-7.dll libbrotlicommon.dll libbrotlidec.dll

echo Building ...
go build -ldflags "-H=windowsgui -s -w"
rem go build

if errorlevel 0 ( 
	echo Build OK 
	) else (
	echo "ERROR build"
	@pause 
	exit /b 1
)

echo Copy libs ...
if not exist %libdir% (
	 md %libdir%
)

for  %%l in (%libs%) do (
	xcopy %mingw%\bin\%%l %libdir%
rem	echo errorlevel %errorlevel%
rem	if errorlevel 0 ( 
rem		echo "%%l copy OK" 
rem	) else (
rem		echo "ERROR copy %%l"
rem	)
)

echo Create etc\gtk-3.0\settings.ini ...
set confdir=%builddir%\etc\gtk-3.0
if not exist %confdir% (
	md %confdir%
)
echo [Settings] > %confdir%\settings.ini
echo gtk-theme-name=Windows10 >> %confdir%\settings.ini
echo gtk-font-name=Segoe UI 9 >> %confdir%\settings.ini


echo Copy Win10 themas ...
%sevenz% x "%CD%\pkg\gtk-3.20.7z" -o"%builddir%\share\themes\Windows10\gtk-3.0\"


echo Create Archive
cd %builddir%\..\
%sevenz% a "%builddir%\..\..\%proj%_Win64_%today%.7z" "%proj%" "-xr!cfg.ini"

@pause
exit /b 0
