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

/*

SYNOPSIS:
	Goscore is designed to offer a simple scoreboard solution for
	cyber security competitions and comes ready to be deployed for a
	competition. It allows specifying services to test in a config
	file, the interval by which to test them on, and the method by
	which to test them; including host level commands that can be
	run and evaluated to determine the services state or by
	manually passing a connection string to the remote services port.
	This program also offers a built in HTML scoreboard with the
	option use your own HTML scoreboard.

	If you are looking for config file help, additional info about
	this program, or are looking for help on creating your own HTML
	scoreboard; see https://github.com/AWildBeard/goscore/wiki

OPTIONS:
	-buildcfg
		This flag will cause the program to write a working config file
		to your current working directory an exit. Use this to generate
		a config template that you can modify to suite your own needs.

	-c [config file]
		This flag allows a user to specify a custom config file location.
		By default, this program checks for the config file in the
		directory where this program is run (your current working
		directory), or the directory where this program is stored.

	-d
		This flag enables debug output to STDERR

	-h
		This flag will display this message and exit.

LICENSE:
	You can view your rights with this software in the LICENSE here:
	https://github.com/AWildBeard/goscore/blob/master/LICENSE and
	can download the source code for this program here:
	https://github.com/AWildBeard/goscore

	By using this piece of software you agree to the terms as they are
	detailed in the LICENSE

	This software is distributed as Free and Open Source Software.

AUTHOR:
	This program was created by Michael Mitchell for the
	University of West Florida Cyber Security Club and includes
	libraries and software written by Canonical, and Cameron Sparr
*/
package main
