function generate_svensson_edb2(toolbox_dir, output_file)
% Generate the finite-wedge reference fixture with the official EDB2 toolbox.
%
% Example:
%   octave --quiet --eval "addpath('.'); generate_svensson_edb2('/path/to/EDB2toolbox', 'output.csv')"

addpath(toolbox_dir);

% Octave's quadgk defaults to 650 intervals, which is insufficient for the
% official toolbox tolerance at 5 and 10 kHz. Keep the EDB2 formula unchanged,
% but raise that implementation limit in a temporary copy of its driver.
if exist('OCTAVE_VERSION', 'builtin')
  source_file = fullfile(toolbox_dir, 'EDB2wedge1st_fd.m');
  source_text = fileread(source_file);
  needle = ',''RelTol'',tol)';
  replacement = ',''RelTol'',tol,''MaxIntervalCount'',10000)';
  patched_text = strrep(source_text, needle, replacement);
  if strcmp(source_text, patched_text)
    error('EDB2wedge1st_fd.m did not contain the expected quadgk call');
  end

  compatibility_dir = tempname();
  mkdir(compatibility_dir);
  compatibility_file = fullfile(compatibility_dir, 'EDB2wedge1st_fd.m');
  compatibility_fid = fopen(compatibility_file, 'w');
  if compatibility_fid < 0
    error('cannot create Octave compatibility copy');
  end
  fprintf(compatibility_fid, '%s', patched_text);
  fclose(compatibility_fid);
  compatibility_cleanup = onCleanup(@() rmdir(compatibility_dir, 's'));
  addpath(compatibility_dir, '-begin');
end

global CAIR;
CAIR = 343.0;

frequencies = [50.0; 500.0; 5000.0; 10000.0];
receiver_azimuths = (-84.0:12.0:84.0).';
closed_wedge_angle = 10.0 * pi / 180.0;
source_theta = 90.0 * pi / 180.0;
edge_ends = [-50.0 50.0];

fid = fopen(output_file, 'w');
if fid < 0
  error('cannot open output file: %s', output_file);
end
output_cleanup = onCleanup(@() fclose(fid));
fprintf(fid, 'receiver_azimuth_deg,frequency_hz,transfer_real,transfer_imag,magnitude_db\n');

for receiver_index = 1:numel(receiver_azimuths)
  receiver_azimuth = receiver_azimuths(receiver_index);
  receiver_theta = receiver_azimuth * pi / 180.0;
  if receiver_theta < 0
    receiver_theta = receiver_theta + 2.0 * pi;
  end

  transfer = EDB2wedge1st_fd( ...
    frequencies, closed_wedge_angle, ...
    10.0, source_theta, 0.0, ...
    10.0, receiver_theta, 0.0, ...
    edge_ends, 'New');

  for frequency_index = 1:numel(frequencies)
    value = transfer(frequency_index);
    fprintf(fid, '%.17g,%.17g,%.17g,%.17g,%.17g\n', ...
      receiver_azimuth, frequencies(frequency_index), ...
      real(value), imag(value), 20.0 * log10(abs(value)));
  end
end

end
