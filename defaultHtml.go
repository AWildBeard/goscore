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
	adminLoginPage = `<!DOCTYPE html>
<html>

<head>
  <meta charset="utf-8">
  <title>Admin Login Page</title>
  <meta name="generator" content="Google Web Designer 5.0.4.0226">
  <style type="text/css" id="gwd-text-style">
    p {
      margin: 0px;
    }
    h1 {
      margin: 0px;
    }
    h2 {
      margin: 0px;
    }
    h3 {
      margin: 0px;
    }
  </style>
  <style type="text/css">
    html, body {
      width: 100%;
      height: 100%;
      margin: 0px;
    }
    body {
      transform: perspective(1400px) matrix3d(1, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1);
      transform-style: preserve-3d;
      background-image: none;
      background-color: rgb(19, 63, 124);
      left: 0%;
      top: 0%;
    }
    .gwd-p-jznn {
      height: auto;
      left: 0px;
      position: absolute;
      top: 0px;
      width: auto;
    }
    [data-gwd-group="LoginForm"] .gwd-grp-mr5o.gwd-form-1qp1 {
      position: absolute;
      left: 0%;
      top: 0%;
      width: 100%;
      height: 100%;
      background-image: none;
      background-color: rgb(255, 255, 255);
      border-style: none;
      border-width: 0px;
      border-radius: 3%;
    }
    [data-gwd-group="LoginForm"] .gwd-grp-mr5o.gwd-p-twd2 {
      position: absolute;
      text-align: center;
      font-family: Arial;
      width: 48.41%;
      height: 9.57%;
      transform-origin: 121.345px 15.7112px 0px;
      left: 26%;
      top: 16%;
    }
    [data-gwd-group="LoginForm"] .gwd-grp-mr5o.gwd-button-lr3j {
      position: absolute;
      padding: 0px;
      width: 19.92%;
      height: 7.41%;
      left: 40%;
      top: 76%;
    }
    [data-gwd-group="LoginForm"] .gwd-grp-mr5o.gwd-input-qdii {
      position: absolute;
      width: 51%;
      height: 6.17%;
      transform-origin: 129.735px 12.6553px 0px;
      left: 24%;
      top: 37%;
    }
    [data-gwd-group="LoginForm"] .gwd-grp-mr5o.gwd-input-ptw7 {
      position: absolute;
      width: 51%;
      height: 6.17%;
      transform-origin: 129.792px 12.7362px 0px;
      left: 24%;
      top: 55%;
    }
    [data-gwd-group="LoginForm"] {
      width: 992px;
      height: 610px;
    }
    .gwd-div-1mdw {
      position: absolute;
      width: 40%;
      height: 40%;
      top: 30%;
      left: 30.04%;
    }
  </style>
    <script>
(function(){var m=function(c){var b=0;return function(){return b<c.length?{done:!1,value:c[b++]}:{done:!0}}},q=function(c){var b="undefined"!=typeof Symbol&&Symbol.iterator&&c[Symbol.iterator];return b?b.call(c):{next:m(c)}},r=function(c){for(var b,d=[];!(b=c.next()).done;)d.push(b.value);return d};var t=function(c){var b=0,d;for(d in c)b++;return b},v=function(c,b){return null!==c&&b in c},w=function(c,b){b in c&&delete c[b]};var x=function(c){c&&c.parentNode&&c.parentNode.removeChild(c)};var z=function(c,b,d,e){var a={m:d,index:e.index,j:e.index};e.i[d]=a;e.h.push(a);e.l[d]=!0;e.index++;d=b[d];for(var g=d.c.length-1;0<=g;g--){var h=d.c[g];v(e.i,h.b)?v(e.l,h.b)&&a.j>e.i[h.b].j&&(x(h.g.a),d.c.splice(g,1)):z(c,b,h.b,e)}if(a.index==a.j)for(;c=e.h[e.h.length-1],w(e.l,c.m),e.h.pop(),c!=a;);},A=function(c,b){c=q(c.childNodes);for(var d=c.next();!d.done;d=c.next())b.appendChild(d.value.cloneNode(!0))};document.addEventListener("DOMContentLoaded",function B(){document.removeEventListener("DOMContentLoaded",B,!1);var b=document,d={},e;if("content"in document.createElement("template")){var a=b.getElementById("gwd-group-definitions");a&&(e=document.importNode(a.content,!0).querySelectorAll("[data-gwd-group-def]"))}e||(e=b.querySelectorAll("[data-gwd-group-def]"));e=q(e);for(a=e.next();!a.done;a=e.next()){a=a.value;var g=a.getAttribute("data-gwd-group-def");g?d[g]?x(a):d[g]={a:a,c:[]}:x(a)}e=[];a=Array.prototype.slice.call(b.querySelectorAll("[data-gwd-group]"));
if("content"in document.createElement("template")&&b.getElementById("gwd-group-definitions"))for(var h in d)g=Array.prototype.slice.call(d[h].a.querySelectorAll("[data-gwd-group]")),0<g.length&&a.push.apply(a,g instanceof Array?g:r(q(g)));h=q(a);for(a=h.next();!a.done;a=h.next()){a=a.value;g=a.childNodes;for(var l=g.length-1;0<=l;l--)8!=g[l].nodeType&&a.removeChild(g[l]);a={a:a,f:a.getAttribute("data-gwd-group")};if(g=d[a.f]){l=a.a.parentNode;for(var y=!1;l&&l!=b;){var u=l.getAttribute("data-gwd-group-def");
if(u&&d[u]){g.c.push({g:a,b:u});y=!0;break}l=l.parentNode}y||e.push(a)}else x(a.a)}h={index:0,i:{},h:[],l:{}};for(var n in d)v(h.i,n)||z(b,d,n,h);b={};for(var f in d)b[f]={};for(var p in d)for(n=q(d[p].c),f=n.next();!f.done;f=n.next())f=b[f.value.b],p in f?f[p]++:f[p]=1;p=[];for(var k in b)0==t(b[k])&&(p.push(k),w(b,k));k=q(p);for(f=k.next();!f.done;f=k.next())for(n=d[f.value],h=q(n.c),f=h.next();!f.done;f=h.next())f=f.value,A(n.a,f.g.a),a=b[f.b],a[f.g.f]--,0==a[f.g.f]&&(w(a,f.g.f),0==t(a)&&(p.push(f.b),
w(b,f.b)));e=q(e);for(k=e.next();!k.done;k=e.next())k=k.value,A(d[k.f].a,k.a)},!1);}).call(this);
    </script>
</head>

<body class="htmlNoPages">
  <template id="gwd-group-definitions">
    <div data-gwd-group-def="LoginForm" data-gwd-group-class="gwd-grp-mr5o" style="display: none;">
      <form method="post" action="/admin" class="gwd-form-1qp1 gwd-grp-mr5o">
        <p class="gwd-p-twd2 gwd-grp-mr5o">Goscore Management Login Page</p>
        <button type="submit" class="gwd-button-lr3j gwd-grp-mr5o" data-gwd-name="loginButton" data-gwd-grp-id="button_2">Login</button>
        <input name="username" type="text" class="gwd-input-qdii gwd-grp-mr5o" data-gwd-name="username" placeholder="Username" data-gwd-grp-id="username_1">
        <input name="password" type="password" class="gwd-input-ptw7 gwd-grp-mr5o" data-gwd-name="password" placeholder="Password" data-gwd-grp-id="password_1">
      </form>
    </div>
  </template>
  <div class="gwd-div-1mdw" data-gwd-group="LoginForm"></div>
</body>

</html>

`
)
