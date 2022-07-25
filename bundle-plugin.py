#!/usr/bin/env python3

import os
import json
import shutil
import configparser
from datetime import datetime, timezone


def bundle_for_platform(lib_file, platform, metadata):
    target_dir = os.path.join('dist', 'plugin', platform, 'gisquick')
    dest_dir = os.path.join('dist', 'plugin', platform, 'gisquick', 'gisquick')

    shutil.copytree('python/', dest_dir, ignore=shutil.ignore_patterns('__pycache__'))
    shutil.copy(os.path.join('dist', 'lib', lib_file), dest_dir)
    name = 'gisquick.%s_%s' % (platform, metadata['version'])
    shutil.make_archive(os.path.join(target_dir, name), 'zip', target_dir, 'gisquick')
    filename = '%s.zip' % name
    metadata = dict(metadata)
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
    config = configparser.ConfigParser()
    config.optionxform = str
    config.read('python/metadata.txt')
    metadata = dict(config.items('general'))
    # metadata['updated'] = datetime.now().isoformat()
    metadata['updated'] = datetime.now(timezone.utc).isoformat().replace('+00:00', 'Z') # datetime.now().isoformat()
    
    shutil.rmtree('dist/plugin', ignore_errors=True)

    # meta = get_metadata(config)
    # print(meta)
    # metadata = {
    #     'version': version
    # }
    # with open('dist/metadata.json', 'w') as outfile:
    #     json.dump(metadata, outfile)

    bundle_for_platform('linux_amd64/gisquick.so', 'lin64', metadata)
    bundle_for_platform('windows_amd64/gisquick.dll', 'win64', metadata)
    bundle_for_platform('darwin_amd64/gisquick.dylib', 'mac64', metadata)
