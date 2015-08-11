package("ui", function(ui){
	depends("/ui/wanderer.js");

	var view =  document.getElementById("view");
	var context = view.getContext("2d");

	function render(){
		context.clearRect(0, 0, view.width, view.height);
		ui.wanderer.renderTo(context);
		ui.loop = requestAnimationFrame(render);
	}
	ui.loop && cancelAnimationFrame(ui.loop);
	ui.loop = requestAnimationFrame(render);
});