hostname {{ .ShortName }}
username admin privilege 15 secret admin
!
interface Management0
{{ if .MgmtIPv4Address }}ip address {{ .MgmtIPv4Address }}/{{.MgmtIPv4PrefixLength}}{{end}}
{{ if .MgmtIPv6Address }}ipv6 address {{ .MgmtIPv6Address }}/{{.MgmtIPv6PrefixLength}}{{end}}
!
management api gnmi
   transport grpc default
!
management api netconf
   transport ssh default
!
management api http-commands
   no shutdown
!
end