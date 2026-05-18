import $ from 'jquery';

export function initRepoUAPFImport() {
  const $open = $('#uapf-import-open');
  if (!$open.length) return;

  // The dropdown only exists on the populated repo view; .dropdown() on an
  // empty set is a harmless no-op, so the trigger also works on the empty repo.
  $('#uapf-import-dropdown').dropdown();

  $open.on('click', (event) => {
    event.preventDefault();
    $('#uapf-import-modal').modal('show');
  });
}
