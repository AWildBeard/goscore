// Copyright 2019 Michael Mitchell
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

const (
	standardScoreboardDoc = `<!DOCTYPE HTML>
<html>
	<head>
		<meta charset="UTF-8">
		<title>{{ .Title }}</title>
		<style>
body {
  display: flex;
  font-family: arial, serif;
  justify-content: center;
  background-color: #133f7c;
  height: 100%;
  margin: 0;
  padding: 0;
  insets: 0;
}
h2 {
  margin: 5vh 0 0 0;
  display: flex;
  flex: 0;
  justify-content: center;
}
.serviceTable {
  height: calc(100vh - 4vh);
  padding: 0 10vw 0 10vw;
  display: flex;
  justify-content: flex-start;
  margin: 2vh 0 2vh 0;
  flex-direction: column;
  background-color: white;
  border-radius: 2vmin;
  box-shadow: 0 0 1vmin #133f7c;
}
.footer {
  display: flex;
  flex: 2;
  align-self: flex-end;
  width: 100%;
  justify-content: center;
  font-size: 10pt;
}
.footer i {
  align-self: flex-end;
  margin: 3vh 0 3vh 0;
}
.serviceTable table {
  display: flex;
  flex: 1;
  margin: 5vh 0 0 0;
  justify-content: center;
  border-collapse: collapse;
  border: none;
}
.serviceTable th {
  border: solid thin black;
  background-color: black;
  color: white;
  padding: 0.5vmax 3vmax 0.5vmax 3vmax;
  vertical-align: middle;
}
.serviceTable td {
  border: solid thin black;
  vertical-align: top;
  padding: 0.5vh 1vw;
}
.up {
  background-color: green;
}
.down {
  background-color: red;
}
		</style>
		<meta http-equiv="refresh" content="5" />
	</head>
	<body>
		<div class="serviceTable">
		<h2>{{ .Title }} Scoreboard</h2>
		<h2>Time Left: {{ FormatDuration .TimeLeft }}</h2>
		<table>
			<tr>
				<th>Host</th>
				<th>Service</th>
				<th>State</th>
				<th>Uptime</th>
				<th>Downtime</th>
			</tr>{{ $pingHosts := .PingHosts }}{{ range $hostIndex, $host := .Hosts }}{{ range $serviceIndex, $service := $host.Services }} 
			<tr>
				<td>{{ $host.Name }}</td>
				<td>{{ $service.Name }}</td>{{ if $pingHosts }}{{ if and $host.IsUp $service.IsUp }}
				<td class="up">Online</td>{{ else }}
				<td class="down">Offline</td>{{ end }}{{ else }}{{ if $service.IsUp }}
				<td class="up">Online</td>{{ else }}
				<td class="down">Offline</td>{{ end }}{{ end }}
				<td>{{ FormatDuration (Uptime $service) }}</td>
				<td>{{ FormatDuration (Downtime $service) }}</td>
			</tr>{{ end }}{{ end }}
		</table>
		<div class="footer">
		<i>Created by Michael Mitchell for the UWF CyberSecurity Club</i>
		</div>
		</div>
	</body>
</html>
`
)
