$(window).ready(function () {
  $("a").each(hijackLocalLink);

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
      loadDocument(a.href);
      history.pushState({ href: a.href }, '', a.href);
      return (false);
    })
  }
}

function loadDocument(href) {
  $('nav#toc a').blur().removeClass('active');
  $.get(href + '?partial', function (data) {
    $('nav#toc a').each(function (i, a) {
      if (a.href == href) {
        $(a).addClass('active');
        $(a).get(0).scrollIntoView({ behavior: 'smooth', block: 'center' });
      }
    })
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
