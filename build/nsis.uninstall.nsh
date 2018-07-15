Section "Uninstall"
  # uninstall for all users
  setShellVarContext all

  # Delete (optionally) installed files
  {{range $}}Delete $INSTDIR\{{.}}
  {{end}}
  Delete $INSTDIR\uninstall.exe

  # Delete install directory
  rmDir $INSTDIR

  # Delete start menu launcher
  Delete "$SMPROGRAMS\${APPNAME}\${APPNAME}.lnk"
  Delete "$SMPROGRAMS\${APPNAME}\Attach.lnk"
  Delete "$SMPROGRAMS\${APPNAME}\Uninstall.lnk"
  rmDir "$SMPROGRAMS\${APPNAME}"

  # Firewall - remove rules if exists
  SimpleFC::AdvRemoveRule "Siotchain incoming peers (TCP:30303)"
  SimpleFC::AdvRemoveRule "Siotchain outgoing peers (TCP:30303)"
  SimpleFC::AdvRemoveRule "Siotchain UDP discovery (UDP:30303)"

  # Remove IPC endpoint
  ${un.EnvVarUpdate} $0 "SIOTCHAIN_SOCKET" "R" "HKLM" "\\.\pipe\siotchain.ipc"

  # Remove install directory from PATH
  ${un.EnvVarUpdate} $0 "PATH" "R" "HKLM" $INSTDIR

  # Cleanup registry (deletes all sub keys)
  DeleteRegKey HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\${GROUPNAME} ${APPNAME}"
SectionEnd
