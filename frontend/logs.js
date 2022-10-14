function getQueryVariable(variable) {
	let query = window.location.search.substring(1);
	let vars = query.split("&");
	for (let i=0;i<vars.length;i++) {
			let pair = vars[i].split("=");
			if(pair[0] == variable){return pair[1];}
	}
	return false;
}

function connect(){
	let cluster = getQueryVariable("cluster")
	let namespace = getQueryVariable("namespace")
	let pod = getQueryVariable("pod")
	let container = getQueryVariable("container")
	let tail = getQueryVariable("tail")
	let follow = getQueryVariable("follow")
	if (namespace == false) {
		namespace="default"
	}
	console.log(cluster, namespace ,pod ,container, tail, follow)
	if (cluster == false || pod == false || container == false) {
		alert("无法获取到容器，请联系管理员")
		return
	}
	let url = "ws://" + document.location.host + "/ws/" + cluster + "/" + namespace + "/" + pod + "/" + container + "/logs?"
	if (tail) {
		url += "&tail="+tail
	}
	if (follow) {
		url += "&follow="+follow
	}

	console.log(url);
	let term = new Terminal({
		// "cursorBlink":true,
	});
	if (window["WebSocket"]) {
		term.open(document.getElementById("terminal"));
		// term.write("logs "+ pod + "...");
		term.toggleFullScreen(true);
		term.fit();
		term.on('data', function (data) {
			conn.send(data)
		});
		conn = new WebSocket(url);
		conn.onopen = function(e) {
		};
		conn.onmessage = function(event) {
			term.writeln(event.data)
			// term.write(event.data)
		};
		conn.onclose = function(event) {
			if (event.wasClean) {
				console.log(`[close] Connection closed cleanly, code=${event.code} reason=${event.reason}`);
			} else {
				console.log('[close] Connection died');
				term.writeln("")
			}
			// term.write('Connection Reset By Peer! Try Refresh.');
		};
		conn.onerror = function(error) {
			console.log('[error] Connection error');
			term.write("error: "+error.message);
			term.destroy();
		};
	} else {
		var item = document.getElementById("terminal");
		item.innerHTML = "<h2>Your browser does not support WebSockets.</h2>";
	}
}
