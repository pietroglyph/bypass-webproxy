'use strict';

var hidden = true
var sandboxAttrState = {
  "allow-scripts": true
}
$(document).ready(function() { // Register callbacks once we're ready
  var url = document.getElementById("input-url");

  // Load the URL in the iframe when the "Go" button is clicked
  $("#input-go").click(function() {
    loadURL(url.value,true);
    return false;
  })
  // Load the URL in a new tab when the open in new tab button is clicked
  $("#input-open").click(function() {
    loadURL(url.value,false);
    return false;
  });

  // Redirect the page to the URL when the open
  // Disable the button if there isn't anything in the input box
  $("#input-url").keypress(function(e) {
    var code = e.keyCode || e.which; // For cross-browser inconsistencies

    // Load the URL when the enter key is pressed
    if (code == 13 && url.value != "") {
      loadURL(url.value,true);
      return false; // Make sure that we don't accidentally submit a form
    }
  });

  $("#input-url").val("");

  // Enable/disable the buttons next to the input, this happens once the text is actually changed
  $("#input-url").on("input", function(e) { // This doesn't support < IE9... Too bad.
    if (e.target.value != "") {
      $("*.input-side").removeClass("disabled");
    } else {
      $("*.input-side").addClass("disabled");
    }
  });

  // Keep the iframe sized to fit below the header seamlessly (without non-iframe overflow)
  $(window).resize(function() {
    if (!hidden) {
      $("#content-frame").height($(window).height() - $("#main-container").height() - 1);
    }
  });

  // Detect anything with the ID opt getting changed (checked/unchecked)
  $("*#opt").change(function() {
    sandboxAttrState[$(this).data("attr-value")] = this.checked; // sandboxAttrState keeps track of the value of these
    var attrString = "";
    // Serialize the entire sandboxAttrState for the sandbox attribute of the iframe
    for (var key in sandboxAttrState) {
      if (sandboxAttrState.hasOwnProperty(key) && sandboxAttrState[key]) { // hasOwnProperty is because JavaScript is weakly typed and we could get a type we don't want
        attrString += key + " ";
      }
    }
    // Totally reset the sandbox attribue to our new serialized value
    $("#content-frame").attr("sandbox", attrString);
  });
});


function loadURL(targurl, inFrame) {
  if (targurl != "") {
    if (hidden && inFrame) {
      hidden = false;
      $(".page-header").slideUp(500, function() {
        $(".jumbotron").css({
          backgroundColor: "rgba(0,0,0,0)",
          padding: "0px",
          marginBottom: "2px"
        });
        $("#content-frame").height($(window).height() - $("#main-container").height() - 1);
      }); // Get rid of the header, then make the jumbotron small once that's done
      $("#content-frame").css({
        width: "100%",
        border: "none",
      });
      $("#content-frame").removeClass("hidden");
    }
    let encodedTargetURL = window.btoa(targurl); // Encode the URL in Base64
    let url = new URL(window.location.href);
    url.pathname = "/p/";
    url.searchParams.set("u", encodedTargetURL);

    if (inFrame) {
      $("#content-frame").attr("src", url);
    } else {
      window.open(url,"_blank")
    }
  }
}
