package livepkg

// jsreloader is the default file reloader
const jsreloader = `
var Reloader = {};
(function(Reloader){
	"use strict";

	Reloader.ReloadAfter = 2000;

	var xhr = new XMLHttpRequest();
	var src = (PkgJSON || document.currentScript.src.replace(".js", ".json"));
	xhr.open("GET", src);

	xhr.onreadystatechange = function(){
		if(xhr.readyState !== 4){ return; }

		if(xhr.status != 200){
			window.setTimeout(window.location.reload, Reloader.ReloadAfter);
			return;
		}

		var result = JSON.parse(xhr.responseText);
		LoadFiles(result.files);

		if(typeof WebSocket !== 'undefined'){
			ListenChanges(result.live);
		}
	};

	xhr.send();


	var loading = {};
	var unloaded = [];

	Reloader.loading = loading;
	Reloader.unloaded = unloaded;

	function LoadFiles(files){
		unloaded = files;
		flush();

		var loadMonitor = window.setInterval(flush, 100);

		// tries to load as many unblocked files as possible
		function flush(){
			// clear loaded
			for(var name in loading){
				var asset = document.getElementById("~" + name);
				if(typeof asset === 'undefined'){ continue; }
				if((asset.readyState === 'complete') ||
					(asset.readyState === 'loaded')){
					delete loading[name];
				}
			}

			// try load something new
			while(unloaded.length > 0){
				if(!tryLoad(unloaded[0])){
					return;
				};
				unloaded.shift();
			}

			// done?
			if(Object.keys(loading).length == 0){
				console.log("reloader", "+++");
				window.clearInterval(loadMonitor);
			}
		}

		// tries to load a file, returns true if it started loading
		function tryLoad(file){
			for(var i = 0; i < file.deps.length; i += 1){
				if(loading[file.deps[i]]){ return false; }
			}

			console.log("reloader", "+ " + file.path);
			loading[file.path] = true;
			var asset = injectFile(file);
			asset.onreadystatechange = flush;

			asset.onload = function(){
				delete loading[file.path];
				flush();
			};
			asset.onerror = function(){
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
		document.getElementsByTagName('head')[0].appendChild(asset);
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
			console.log("reloader", "%", change);
			Reloader.Change && Reloader.Change(change);
			Reloader.onchange && Reloader.onchange(change);
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
				removeFile(change.prev);
			} else {
				var asset = swapFile(change.prev, change.next);
				asset.onload = onFileChanged(change);
			}
		});

		ws.addEventListener('open', function(){
			console.log("livepkg connected");
			if(OnceConnected){
				// server changed, force reload
				window.location.reload();
			}
			OnceConnected = true;
		});

		ws.addEventListener('close', function(ev){
			console.log("livepkg disconnected", ev);
			window.setTimeout(function(){ ListenChanges(livepath); }, ConnectionDelay);
			ConnectionDelay *= 2;
			if(ConnectionDelay > 5000){
				ConnectionDelay = 5000;
			}
		});
	}
})(Reloader);
`
