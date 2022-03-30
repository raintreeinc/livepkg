package livepkg

// jspackage is the default packages manager
const jspackage = `
(function(global){
	global.packages = packages;
	function packages(name, setup){
		if(name == ""){
			throw new Error("packages name cannot be empty");
		}

		var info = packages.find(name);
		if(packages.debug){
			if(info.created){
				console.log("loading: ", name);
			} else {
				console.log("reloading: ", name);
			}
		}
		var exports = setup(info.namespace);
		if(exports !== undefined){
			for(var name in exports){
				if(exports.hasOwnProperty(name)){
					info.namespace[name] = exports[name];
				}
			}
		}
	}

	packages.debug = false;

	packages.find = function find(name){
		var created = false;
		var path = name.split(".");
		var namespace = global;

		for(var i = 0; i < path.length; i++){
			var token = path[i];
			var next = namespace[token];
			if(next){
				created = false
			} else {
				next = {};
				namespace[token] = next;
				created = true;
			}
			namespace = next;
		}

		return {
			namespace: namespace,
			created: created
		};
	}

	global.depends = function depends(filename){};
})(window || this);
`
