import $ from 'jquery';

export function initRepoUAPFImport() {
  const $dropdown = $('#uapf-import-dropdown');
  if (!$dropdown.length) return;

  $dropdown.dropdown();

  $('#uapf-import-open').on('click', (event) => {
    event.preventDefault();
    $('#uapf-import-modal').modal('show');
  });
}
