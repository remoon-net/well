!include "version.nsh"

Name "well-net"
OutFile "well-net-${VERSION}-setup.exe"
InstallDir "$PROGRAMFILES\well-net"
RequestExecutionLevel admin

Var DataDir

Section "Install"
    ; -----------------------------
    ; 数据目录（硬编码）
    ; -----------------------------
    StrCpy $DataDir "C:\ProgramData\well-net"
    ; 如果目录不存在则创建
    IfFileExists "$DataDir\*" 0 +2
        Goto DataDirExists
    CreateDirectory "$DataDir"
    CreateDirectory "$DataDir\logs"
DataDirExists:

    ; -----------------------------
    ; 安装目录初始化
    ; -----------------------------
    SetOutPath "$INSTDIR"
    CreateDirectory "$INSTDIR"

    ; -----------------------------
    ; 停止并删除旧服务
    ; -----------------------------
    ExecWait '"$INSTDIR\nssm.exe" stop well-net'
    ExecWait '"$INSTDIR\nssm.exe" remove well-net confirm'

    ; -----------------------------
    ; 拷贝程序文件
    ; -----------------------------
    File "well-net.exe"
    File "nssm.exe"
    File "wintun.dll"

    ; -----------------------------
    ; 创建卸载程序
    ; -----------------------------
    WriteUninstaller "$INSTDIR\Uninstall.exe"

    ; -----------------------------
    ; 注册服务
    ; -----------------------------
    ExecWait '"$INSTDIR\nssm.exe" install well-net "$INSTDIR\well-net.exe"'
    ExecWait '"$INSTDIR\nssm.exe" set well-net DependOnService Tcpip'
    ExecWait '"$INSTDIR\nssm.exe" set well-net AppDirectory "$INSTDIR"'
    ExecWait '"$INSTDIR\nssm.exe" set well-net AppParameters "serve --dir $DataDir"'
    ExecWait '"$INSTDIR\nssm.exe" set well-net AppStdout "$DataDir\logs\stdout.log"'
    ExecWait '"$INSTDIR\nssm.exe" set well-net AppStderr "$DataDir\logs\stderr.log"'
    ExecWait '"$INSTDIR\nssm.exe" set well-net AppExit Default Restart'
    ExecWait '"$INSTDIR\nssm.exe" set well-net Start SERVICE_AUTO_START'

    ; -----------------------------
    ; 启动服务
    ; -----------------------------
    Exec '"$INSTDIR\nssm.exe" start well-net'

    ; -----------------------------
    ; 卸载信息注册表
    ; -----------------------------
    WriteRegStr HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\well-net" "DisplayName" "well-net"
    WriteRegStr HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\well-net" "Publisher" "ReMoon"
    WriteRegStr HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\well-net" "DisplayVersion" "${VERSION}"
    WriteRegStr HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\well-net" "InstallLocation" "$INSTDIR"
    WriteRegStr HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\well-net" "UninstallString" "$INSTDIR\Uninstall.exe"
    WriteRegStr HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\well-net" "DisplayIcon" "$INSTDIR\well-net.exe"
SectionEnd

Section "Uninstall"
    ; -----------------------------
    ; 数据目录（硬编码）
    ; -----------------------------
    StrCpy $DataDir "C:\ProgramData\well-net"

    ; -----------------------------
    ; 停止并删除服务
    ; -----------------------------
    Exec '"$INSTDIR\nssm.exe" stop well-net'
    Exec '"$INSTDIR\nssm.exe" remove well-net confirm'

    ; -----------------------------
    ; 删除程序文件和安装目录
    ; -----------------------------
    Delete "$INSTDIR\well-net.exe"
    Delete "$INSTDIR\nssm.exe"
    Delete "$INSTDIR\Uninstall.exe"
    Delete "$INSTDIR\wintun.dll"
    RMDir "$INSTDIR"

    ; -----------------------------
    ; （可选）删除数据目录
    ; -----------------------------
    ; RMDir /r "$DataDir"  ; 注释掉保留用户数据

    ; -----------------------------
    ; 删除卸载注册表
    ; -----------------------------
    DeleteRegKey HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\well-net"
SectionEnd
