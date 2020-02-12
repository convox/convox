$(window).ready(function () {
  $("a").each(hijackLocalLink);

  scrollActiveToc();
  addHeadingAnchors();
});

$(window).on('popstate', function (e) {
  if (e.originalEvent.state) {
    loadDocument(e.originalEvent.state.href)
  } else {
    loadDocument(window.location.href);
  }
})

function addHeadingAnchors() {
  $("h2, h3, h4, h5, h6").each(function (i, el) {
    $(el).prepend($('<a class="heading-link"><i class="fa fa-link"></a>').attr("href", "#" + $(el).attr("id")));
  });
}

function hijackLocalLink(i, a) {
  if (window.location.host == a.host) {
    $(a).on('click', function (e) {
      if (e.ctrlKey || e.shiftKey || e.metaKey || (e.button && e.button == 1)) return;
      loadDocument(a.href);
      history.pushState({ href: a.href }, '', a.href);
      return (false);
    })
  }
}

function loadDocument(href) {
  $('nav#toc a').blur().removeClass('active');
  var url = new URL(href);
  $.get('/toc' + url.pathname, function (data) {
    $('nav#toc').html(data);
    scrollActiveToc();
  });
  $.get(href + '?partial', function (data) {
    $('main').html(data);
    $('main').scrollTop(0);
    $('main a').each(hijackLocalLink);
    addHeadingAnchors();
    var anchor = href.split('#')[1];
    if (anchor) {
      var anchorlink = $(`main a[href="#${anchor}"]`).get(0)
      if (anchorlink) {
        anchorlink.scrollIntoView({ behavior: 'smooth', block: 'start' });
      }
    }
  })

}

function queryChange() {
  if ($('#search-query').val() != '') {
    queryShow();
  }
}

function queryClose() {
  window.setTimeout(function () { $('#search-hits').hide(200); }, 500);
}

function queryShow() {
  $('#search-hits').show(200);
}

function scrollActiveToc() {
  var active = $('nav#toc a.active').get(0);
  if (active) {
    active.scrollIntoView({ block: 'center' });
  }
}
