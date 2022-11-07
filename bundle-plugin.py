#!/usr/bin/env python3

import os
import json
import shutil
import configparser
from datetime import datetime, timezone


class Target:
    def __init__(self, platform, arch, lib_suffix, executable_sufix):
        self.platform = platform
        self.arch = arch
        self.lib_suffix = lib_suffix
        self.executable_sufix = executable_sufix

Targets = {
    'lin64': Target('lin64', 'linux_amd64', '.so', ''),
    'win64': Target('win64', 'windows_amd64', '.dll', '.exe'),
    'mac64': Target('mac64', 'darwin_amd64', '.dylib', '')
}

def read_metadata(filename):
    config = configparser.ConfigParser()
    config.optionxform = str
    config.read(filename)
    metadata = dict(config.items('general'))
    # metadata['updated'] = datetime.now().isoformat()
    metadata['updated'] = datetime.now(timezone.utc).isoformat().replace('+00:00', 'Z') # datetime.now().isoformat()
    return metadata

def create_dbhash_version(dest_dir):
    config = configparser.ConfigParser()
    config.optionxform = str
    meta_filename = os.path.join(dest_dir, 'metadata.txt')
    config.read(meta_filename)
    config['general']['name'] += ' (with dbhash)'
    config['general']['about'] += """ Version with SQLite's <a href="https://www.sqlite.org/dbhash.html">dbhash</a> program, recommended when working with Geopackage format."""
    with open(meta_filename, 'w') as configfile:
        config.write(configfile)


def bundle_for_platform(target_name, dbhash=False):
    target = Targets[target_name]
    platform = target.platform
    basename = 'gisquick' if not dbhash else 'gisquick_dbhash'
    target_dir = os.path.join('dist', 'plugin', platform, basename)
    dest_dir = os.path.join('dist', 'plugin', platform, basename, basename)

    shutil.copytree('python/', dest_dir, ignore=shutil.ignore_patterns('__pycache__'))
    shutil.copy(os.path.join('dist', 'lib', target.arch, 'gisquick' + target.lib_suffix), dest_dir)
    if dbhash:
        shutil.copy(os.path.join('dbhash', 'dist', target.arch, 'dbhash' + target.executable_sufix), dest_dir)
        create_dbhash_version(dest_dir)

    metadata = read_metadata(os.path.join(dest_dir, 'metadata.txt'))
    name = '%s.%s_%s' % (basename, platform, metadata['version'])
    shutil.make_archive(os.path.join(target_dir, name), 'zip', target_dir, basename)
    filename = '%s.zip' % name
    metadata['filename'] = filename
    icon = metadata.get('icon', None)
    if icon:
        icon_src = os.path.join('python', icon)
        icon_name = os.path.basename(icon)
        icon_dest = os.path.join(target_dir, icon_name)
        shutil.copy(icon_src, icon_dest)
        metadata['icon'] = icon_name
    with open(os.path.join(target_dir, 'metadata.json'), 'w') as outfile:
        json.dump(metadata, outfile)
    shutil.rmtree(dest_dir)


def get_metadata(config):
    sections_dict = {}
    # get sections and iterate over each
    for section in config.sections():
        options = config.options(section)
        temp_dict = {}
        for option in options:
            temp_dict[option] = config.get(section,option)
        
        sections_dict[section] = temp_dict

    return sections_dict


if __name__ == '__main__':
    
    shutil.rmtree('dist/plugin', ignore_errors=True)

    bundle_for_platform('lin64')
    bundle_for_platform('win64')
    bundle_for_platform('mac64')

    bundle_for_platform('lin64', dbhash=True)
    bundle_for_platform('win64', dbhash=True)
    bundle_for_platform('mac64', dbhash=True)
