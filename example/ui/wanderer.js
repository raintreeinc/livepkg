package("ui.wanderer", function(w){
	"use strict";

	w.x = w.x || 0;
	w.y = w.y || 0;

	w.renderTo = function(context){
		var time = (new Date())|0;
		context.fillRect(w.x, w.y, Math.sin(time/1000)*10, Math.sin(time/1000)*10);

		w.x = (w.x + Math.random()*10) % view.width;
		w.y = (w.y + Math.random()*10) % view.height;
	}
});