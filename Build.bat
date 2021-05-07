@echo off
clear

set proj=potbs-chat
set today=%date:~6,4%%date:~3,2%%date:~0,2%
set mingw=C:\msys64\mingw64
set sevenz="C:\Program Files\7-Zip\7z.exe"
set builddir=%CD%\Build\%today%\%proj%
set libdir=%builddir%


echo Building ...
go build -ldflags "-H=windowsgui -s -w"

if errorlevel 0 ( 
	echo Build OK 
	) else (
	echo "ERROR build"
	@pause 
	exit /b 1
)


echo Copy  %proj%.exe...
xcopy %proj%.exe %builddir%\

echo Copy libs ...
if not exist %libdir% (
    md %libdir%
)
ldd %builddir%\%proj%.exe | grep '\/mingw.*\.dll' | awk '{print $3}' | xargs -I '{}' cp -v '{}' %libdir%


echo Copy pixbuf
xcopy %mingw%\lib\gdk-pixbuf-2.0 %builddir%\lib\gdk-pixbuf-2.0\ /S
del /s /q /f %builddir%\lib\gdk-pixbuf-2.0\2.10.0\loaders\*.a


echo Create etc\gtk-3.0\settings.ini ...
set confdir=%builddir%\etc\gtk-3.0
if not exist %confdir% (
    md %confdir%
)

rem echo [Settings]> %confdir%\settings.ini
rem echo gtk-theme-name=Windows10>> %confdir%\settings.ini
rem echo gtk-font-name=Segoe UI 9>> %confdir%\settings.ini
(echo [Settings]
echo gtk-theme-name = win32 
echo gtk-icon-theme-name = Adwaita 
echo gtk-xft-antialias=1
echo gtk-xft-hinting=1
echo gtk-xft-hintstyle=hintfull
echo gtk-xft-rgba=rgb
) > %confdir%\settings.ini


rem echo Copy Win10 themas ...
rem %sevenz% x "%CD%\pkg\gtk-3.20.7z" -o"%builddir%\share\themes\Windows10\gtk-3.0\"

echo Copy Adwaita ...
set adwaita=%mingw%\share\icons\Adwaita
set adwaita_build=%builddir%\share\icons\Adwaita

rem         (16x16,22x22,24x24,32x32,48x48,64x64,96x96,256x256)
for  %%r in (16x16,22x22,24x24,32x32,48x48) do (
	md %adwaita_build%\%%r\actions

	xcopy %adwaita%\%%r\actions\list-add-symbolic.symbolic.png  %adwaita_build%\%%r\actions\
	xcopy %adwaita%\%%r\actions\list-add.png  %adwaita_build%\%%r\actions\
rem	xcopy %adwaita%\%%r\actions\list-remove-all-symbolic.symbolic.png  %adwaita_build%\%%r\actions\
	xcopy %adwaita%\%%r\actions\list-remove-symbolic.symbolic.png  %adwaita_build%\%%r\actions\
	xcopy %adwaita%\%%r\actions\list-remove.png  %adwaita_build%\%%r\actions\
)
md %adwaita_build%\scalable\actions
xcopy %adwaita%\scalable\actions\list-add-symbolic.svg %adwaita_build%\scalable\actions\
xcopy %adwaita%\scalable\actions\list-remove-symbolic.svg %adwaita_build%\scalable\actions\

(echo [Icon Theme]
echo Name=Adwaita
echo Comment=The Only One
echo Example=folder
echo. 
echo # KDE Specific Stuff
echo DisplayDepth=32
echo LinkOverlay=link_overlay
echo LockOverlay=lock_overlay
echo ZipOverlay=zip_overlay
echo DesktopDefault=48
echo DesktopSizes=16,22,32,48,64,72,96,128
echo ToolbarDefault=22
echo ToolbarSizes=16,22,32,48
echo MainToolbarDefault=22
echo MainToolbarSizes=16,22,32,48
echo SmallDefault=16
echo SmallSizes=16
echo PanelDefault=32
echo PanelSizes=16,22,32,48,64,72,96,128
echo. 
echo # Directory list
echo Directories=16x16/actions,22x22/actions,22x22/legacy,24x24/actions,24x24/legacy,32x32/actions,32x32/legacy,48x48/actions,48x48/legacy,scalable/actions,
echo. 
echo [16x16/actions]
echo Context=Actions
echo Size=16
echo Type=Fixed
echo. 
echo [22x2/actions]
echo Context=Actions
echo Size=22
echo Type=Fixed
echo. 
echo [22x22/legacy]
echo Context=Legacy
echo Size=22
echo Type=Fixed
echo. 
echo [24x24/actions]
echo Context=Actions
echo Size=24
echo Type=Fixed
echo. 
echo [24x24/legacy]
echo Context=Legacy
echo Size=24
echo Type=Fixed
echo. 
echo [32x32/actions]
echo Context=Actions
echo Size=32
echo Type=Fixed
echo. 
echo [32x32/legacy]
echo Context=Legacy
echo Size=32
echo Type=Fixed
echo. 
echo [48x48/actions]
echo Context=Actions
echo Size=48
echo Type=Fixed
echo. 
echo [48x48/legacy]
echo Context=Legacy
echo Size=48
echo Type=Fixed
echo. 
echo [scalable/actions]
echo Context=Actions
echo Size=16
echo MinSize=8
echo MaxSize=512
echo Type=Scalable
echo. 
) > %adwaita_build%\index.theme


echo Create Archive
cd %builddir%\..\
%sevenz% a "%builddir%\..\..\%proj%_Win64_%today%.7z" "%proj%" "-xr!cfg.ini"

@pause
exit /b 0
