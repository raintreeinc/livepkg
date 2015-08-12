package livepkg

const jsreloader = `
var Reloader = {};
(function(Reloader){
	"use strict";

	Reloader.ReloadAfter = 2000;

	var xhr = new XMLHttpRequest();
	xhr.open("GET", document.currentScript.src + "on");

	xhr.onload = function(){
		if(xhr.status != 200){
			window.setTimeout(window.location.reload, Reloader.ReloadAfter);
			return;
		}

		var result = JSON.parse(xhr.responseText);
		ListenChanges(result.live);
		LoadFiles(result.files);
	};

	xhr.onerror = function(){
		window.setTimeout(window.location.reload, Reloader.ReloadAfter);
		return;
	}
	xhr.send();


	var loading = {};
	var unloaded = [];
	function LoadFiles(files){
		unloaded = files;
		flush();

		// tries to load as many unblocked files as possible
		function flush(){
			while(unloaded.length > 0){
				if(!tryLoad(unloaded[0])){
					return;
				};
				unloaded.shift();
			}
			if(Object.keys(loading).length == 0){
				console.log("reloader", "All files loaded.");
			}
		}

		// tries to load a file, returns true if it started loading
		function tryLoad(file){
			for(var i = 0; i < file.deps.length; i += 1){
				if(loading[file.deps[i]]){ return false; }
			}
			console.log("reloader", "Loading: " + file.path);
			loading[file.path] = true;
			var asset = injectFile(file);
			asset.onload = function(){
				delete loading[file.path];
				flush();
			};
			return true;
		}
	}

	function makeDOMElement(file){
		switch(file.ext){
		case ".js":
			var asset = document.createElement("script");
			asset.src = file.path + "?" + Math.random();
			break;
		case ".css":
			var asset = document.createElement("link");
			asset.href = file.path + "?" + Math.random();
			asset.rel = "stylesheet";
			break;
		default:
			return;
		}
		asset.id = "~" + file.path;
		return asset;
	}

	function injectFile(file){
		var asset = makeDOMElement(file);
		document.head.appendChild(asset);
		return asset;
	}

	function removeFile(file){
		var previous = document.getElementById("~" + file.path);
		previous.parentNode.removeChild(previous);
	}

	function swapFile(prev, next){
		var next = makeDOMElement(next);
		var prev = document.getElementById(next.id);
		prev.parentNode.insertBefore(next, prev);
		setTimeout(function(){
			prev.parentNode.removeChild(prev);
		}, 20);
		return next;
	}

	function reload(){
		window.location.reload();
	}

	function onFileChanged(change){
		return function(){
			console.log("change", change);
			Reloader.Change && Reloader.Change(change);
		};
	}

	var OnceConnected = false;
	var ConnectionDelay = 100;
	function ListenChanges(livepath){
		if(livepath == null){ return; }
		var ws = new WebSocket("ws://" + window.location.host + livepath);

		ws.addEventListener('message', function(ev){
			if(ev.data === "") { return; }
			var change = JSON.parse(ev.data);
			if(change.deps){
				reload();
				return;
			}

			if(change.prev == null){
				var asset = injectFile(change.next);
				asset.onload = onFileChanged(change);
			} else if(change.next == null){
				remove(change.prev);
			} else {
				var asset = swapFile(change.prev, change.next);
				asset.onload = onFileChanged(change);
			}
		});

		ws.addEventListener('open', function(){
			console.log("livebundle connected");
			if(OnceConnected){
				// server changed, force reload
				window.location.reload();
			}
			OnceConnected = true;
		});

		ws.addEventListener('close', function(ev){
			console.log("livebundle disconnected", ev);
			window.setTimeout(function(){ ListenChanges(livepath); }, ConnectionDelay);
			ConnectionDelay *= 2;
			if(ConnectionDelay > 5000){
				ConnectionDelay = 5000;
			}
		});
	}
})(Reloader);
`